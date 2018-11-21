# Instructions for contributing a provider
Follow these steps to be accepted as a provider within the Virtual Kubelet repo. 

1. Replicate the life-cycle of a pod for example creation and deletion of a pod and how that maps to your service. 
2. Create a new provider folder with a descriptive name and the necessary code. 
3. When committing your code add a README.md, helm chart, dockerfile and specify a maintainer of the provider. 
4. Within the PR itself add a justification for why the provider should be accepted, as well as customer use cases if applicable. 

Some providers are translations of Virtual Kubelet to allow others to adapt their service or applications that are written in other languages. 

# Matrix of current features per provider 

|        | Alibaba ECI | Azure Batch | Azure Container Instances | Hyper.sh | Service Fabric Mesh  | Huawei  | Vic |
|:Features|       :---:              |---|---|---|---|---|---|
|Create pod|   |   | X |   |   |   |   |
|Delete pod|   |   | X  |   |   |   |   |
|Update pod|   |   | X  |   |   |   |   |
|   Logs   |   |   | X |   |   |   |   |
| Metrics  |   |   | X |   |   |   |   |
| Exec     |   |   | X |   |   |   |   |
| Get pod  |   |   | X |   |   |   |   |
|Get pod status|   | X |   |   |   |   |   |
| Capacity  |   |   |   |   |   |   |   |
| Linux |   |   |   | X |   |   |   |
| Windows  |   |   | X |   |   |   |   |
| GPU w/ Linux  |   |   | X |   |   |   |   |
| VNet for Linux  |   |   | X |   |   |   |   |
| VNet for Windows  |   |   |   |   |   |   |   |
| Node Capacity  |   |   |   |   |   |   |   |
|         |   |   |   |   |   |   |   |
