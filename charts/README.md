# The Virtual Kubelet Helm chart

Each version of Virtual Kubelet has a dedicated [Helm](https://helm.sh) chart. Those charts are served as static assets directly from GitHub.

## The `index.yaml` file

This subdirectory has an `index.yaml` file, which is necessary for it to act as a Helm chart repository. To re-generate the `index.yaml` file (assuming that you have Helm installed):

```shell
cd /path/to/virtual-kubelet
helm repo index charts
```

The `index.yaml` then needs to be committed to Git and merged to `master`.
