package webhook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/golang/glog"
	"github.com/google/go-cmp/cmp"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
	// (https://github.com/kubernetes/kubernetes/issues/57982)
	defaulter      = runtime.ObjectDefaulter(runtimeScheme)
	diffCache      = make(map[string]string)
	diffCount      = make(map[string]int)
	diffTotalCount = make(map[string]int)
	traceOnly      = os.Getenv("TRACE_ONLY")
)

const (
	ktest      = "ktest.ibm.com"
	enabled    = "enabled"
	disabled   = "disabled"
	maxDenials = 20
)

// WebhookServer is the webhook server struct
type WebhookServer struct {
	Server *http.Server
}

// WhSvrParameters is Webhook Server parameters
type WhSvrParameters struct {
	Port           int    // webhook server port
	CertFile       string // path to the x509 certificate for https
	KeyFile        string // path to the x509 private key matching `CertFile`
	SidecarCfgFile string // path to sidecar injector configuration file
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1beta1.AddToScheme(runtimeScheme)
	// defaulting with webhooks:
	// https://github.com/kubernetes/kubernetes/issues/57982
	_ = v1.AddToScheme(runtimeScheme)
}

func getDiffKey(name, namespace string) string {
	return name + "-" + namespace
}

// allowed is a function that determines whether or not to allow an update to get through
// If keeps track of diffs in a diffCache, and disallows updates when it sees a diff the first time, allowing the update the second time
// If we find a diff in the diffCache for this name and namespace, it means that the update was not allowed when last seen
func allowed(name, namespace, diff string) bool {
	key := getDiffKey(name, namespace)
	lastDiff := diffCache[key]

	glog.Infof("*****Total denials: %v", diffTotalCount[key])
	if diffTotalCount[key] == maxDenials {
		return true
	}

	if lastDiff == "" {
		glog.Info("***** New diff ^^^^^")
		diffCache[key] = diff
		diffCount[key] = 1
		diffTotalCount[key]++
		return false
	}

	if diff == lastDiff { // This diff was seen before, so we allow it this time and reset the cache
		glog.Info("***** Same diff seen again ^^^^^")
		if diffCount[key] == 1 {
			diffCount[key] = 2
			diffTotalCount[key]++
			return false
		}
		diffCache[key] = ""
		diffCount[key] = 0
		return true
	}

	glog.Info("***** New diff ^^^^^")
	// A new diff has arrived, eventhough the last one didn't get through
	diffCache[key] = diff
	diffCount[key] = 1
	diffTotalCount[key]++
	return false
}

// main mutation process
func (whsvr *WebhookServer) mutate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request

	obj := &unstructured.Unstructured{}
	err := json.Unmarshal(req.Object.Raw, &obj)
	if err != nil {
		// Ignore errors
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	oldObj := &unstructured.Unstructured{}
	err = json.Unmarshal(req.OldObject.Raw, &oldObj)
	if err != nil {
		// Ignore errors
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	diff := cmp.Diff(oldObj.Object, obj.Object)
	name := getName(obj.Object)
	namespace := getNamespace(obj.Object)
	glog.Info("================================")
	glog.Infof("Resource %s in namespace %s: %v", name, namespace, diff)

	if traceOnly == "true" {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	all := allowed(name, namespace, diff)
	if !all {
		glog.Info("**** Operation denied!!!! ****")
		return &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "***** Operation denied by KTest!!!!!! *****",
			},
		}
	} else {
		glog.Info("**** Operation allowed!!!! ****")
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

}

func getName(obj map[string]interface{}) string {
	metadata, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	name, ok := metadata["name"].(string)
	if !ok {
		return ""
	}
	return name
}

func getNamespace(obj map[string]interface{}) string {
	metadata, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	namespace, ok := metadata["namespace"].(string)
	if !ok {
		return ""
	}
	return namespace
}

// Serve method for webhook server
func (whsvr *WebhookServer) Serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		glog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		glog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		glog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		if r.URL.Path == "/mutate" {
			admissionResponse = whsvr.mutate(&ar)
		}
	}

	admissionReview := v1beta1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		glog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	if _, err := w.Write(resp); err != nil {
		glog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}
