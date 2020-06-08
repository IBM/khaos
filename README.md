# KTest

KTest provides a tracing and debugging tool for developing Kubernetes operators.
It is an admission webhook that can be configured to work with specific kinds of resources, and is installed without requiring any modifications to the operator code. 

KTest has 2 modes of operation: trace-only and adverserial testing. In trace-only mode, the webhook captures operations to the specified resources before they reach etcd, and prints out all changes in a human-readable way. This feature can be used to inspect how a resource gets modified over time, with changes from end-users or Kubernetes processes (e.g. controllers).

In adverserial testing mode, KTest traces changes to resources as well, but it also acts like an adversary to get in the way of updates and test the resiliency of an operator. When operators process custom resources, they often request updates to the
resource's status (e.g. through the controller-runtime). These updates can fail for a variety of reasons, including if
another user or process has modified the resource at the same time. KTest mocks these failures by denying updates. It computes
the diff of incoming updates, and denies the same diff twice before accepting it. This stresses the controller to recover
from such failures and progress towards a desired state.


## Installing and usage

Before installing the webhook, you need to configure it to capture desired resources. To do so,
edit the file `hack/templates/mutatingwebhook.yaml` under the section `rules`. The template shows
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

To configure KTest in trace-only mode, edit the file `deploy/000-ktest-config.yaml` and set the `traceonly` field to be `true`, set it to `false`
for tracing and adverserial testing.

Execute the following command to install KTest:

```bash
hack/install-webhook.sh
```

At this point, the webhook will silently intercept all operations on the desired resources and print the changes
to resources in a log. Notice that these operations could have been initiated by a user, or by a Kubernetes controller.

To stream this log, execute the following command:
```bash
hack/trace.sh
```

### Trace-only mode

To configure KTest in trace-only mode, edit the file `deploy/000-ktest-config.yaml` and set the `traceonly` field to be `true` before installation.
Then create, update, delete desired resources and inspect human-readable traces by running the script:

```bash
hack/trace.sh
```

### Adverserial mode

To configure KTest in trace and adverserial testing mode, edit the file `deploy/000-ktest-config.yaml` and set the `traceonly` field to be `false` before installation.

Then create, update, delete desired resources and inspect human-readable traces by running the script:
```bash
hack/trace.sh
```

You will observe that KTest will deny updates periodically before letting them through to etcd. It computes the diff of updates and denies the same diff twice before accepting the update. There is a maximum denial of 20 per resource name/namespace (in the future this parameter will be configurable).

KTest does not currently let you know that a test experiment has failed because it does not know of success/failure conditions. Rather, during your interactions with resources, you can inspect the produced trace and decide if the outcome is correct.

Some outcomes that indicate possible issues with the operator are the following:
- The resource stalls in a state and stops making progress towards the desired state. This could indicate that the reconciler is not being requeued correctly.
- The resource reaches a failed state instead of the desired state. The action of the webhook is adverserial but it should not in itself cause a failure to reach the desired state. If this is the case, it points to a possible bug in the operator. The trace produced by KTest can be used for manual root-cause analysis.
- The controller ends up with incorrect side-effects inside Kubernetes and outside, if any. When updates fail, the reconciler has to run again, so it might incorrectly apply side-effects multiple times.

The maximum denials parameter exists because some operators eventually need a unique update to be successful. Otherwise, KTest would have hindered sucsess indefinitely. This parameter is hard-wired to 20 and will be configurable in the future. After the maximum is reached, the webhook no longer denies any updates on that resource name/namespace. In the future, we plan to provide a feature to reset this parameter. Currently, resetting can be achieved by uninstalling and re-installing the webhook. Notice that the maximum is per custom resource (name and namespace).

KTest mocks update failures but does not produce a trace with an actual failure (meaning that the update failures produce by KTest are artificial). To produce realistic traces, the KTest failures can be replaced by another user modifying the same resource at the same time (which could cause a library like controller-runtime to return an error for an update).



## Uninstalling

To uninstall the webhook, execute the following command:
```bash
hack/uninstall-webhook.sh
```
