Much of this is copied from k8s.io/kubernetes, even if it isn't a 1-1 copy of a
file.  This exists so we do not have to import from k8s.io/kubernetes which is
currently problematic.  Ideally most or all of this will go away and an upstream
solution is found so that we can share an implementation with Kubelet without
importing from k8s.io/kubernetes


| filename | upstream location |
|----------|-------------------|
| envvars.go | https://github.com/kubernetes/kubernetes/blob/98d5dc5d36d34a7ee13368a7893dcb400ec4e566/pkg/kubelet/envvars/envvars.go#L32 |
| helper.go#ConvertDownwardAPIFieldLabel | https://github.com/kubernetes/kubernetes/blob/98d5dc5d36d34a7ee13368a7893dcb400ec4e566/pkg/apis/core/pods/helpers.go#L65 |
| helper.go#ExtractFieldPathAsString | https://github.com/kubernetes/kubernetes/blob/98d5dc5d36d34a7ee13368a7893dcb400ec4e566/pkg/fieldpath/fieldpath.go#L46 |
| helper.go#SplitMaybeSubscriptedPath | https://github.com/kubernetes/kubernetes/blob/98d5dc5d36d34a7ee13368a7893dcb400ec4e566/pkg/fieldpath/fieldpath.go#L96 |
| helper.go#FormatMap | https://github.com/kubernetes/kubernetes/blob/ea0764452222146c47ec826977f49d7001b0ea8c/pkg/fieldpath/fieldpath.go#L29 |
| helper.go#IsServiceIPSet | https://github.com/kubernetes/kubernetes/blob/ea0764452222146c47ec826977f49d7001b0ea8c/pkg/apis/core/v1/helper/helpers.go#L139 |