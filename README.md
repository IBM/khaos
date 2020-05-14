# KTest

KTest provides a tracing tool for debugging Kubernetes operators. It is an admission webhook that can be installed without any
modification to the operator code. The webhook captures all operations to a class of resources, and prints out all changes
to those resources in a human-readable way. 

## Installing and usage

Before installing the webhook, you need to configure it to capture desired resources. To do so,
edit the file `hack/templates/mutatingwebhook.yaml` under the section `rules`. The temlplate shows
an example:

```yaml
 rules:
      - operations: [ "CREATE", "UPDATE", "DELETE" ]
        apiGroups: ["ibmcloud.ibm.com"]
        apiVersions: ["v1alpha1"]
        resources: ["services","services/status"]
```

This indicates the operations that should trigger the webhook (in this case `CREATE`, `UPDATE`, and `DELETE`),
as well as the desired `apiGroups`, `apiVersions`, and `resources` (in this case `services` and subresource `services/status`).

Once this file has been configured, execute the following command to install KTest:

```bash
hack/install-webhook.sh
```

At this point, the webhook will silently intercept all operations on the desired resources and print the changes
to resources in a log. Notice that these operations could have been initiated by a user, or by a Kubernetes controller.

To stream this log, execute the following command:
```bash
hack/trace.sh
```

## Uninstalling

To uninstall the webhook, execute the following command:
```bash
hack/uninstall-webhook.sh
```

