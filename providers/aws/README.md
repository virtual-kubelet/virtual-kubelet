# Kubernetes virtual-kubelet with AWS Fargate

## Prerequisite

This guide assumes that you have a Kubernetes cluster up and running in AWS and that `kubectl` is already configured to talk to it.

Other pre-requesites are:

* The [AWS CLI](https://aws.amazon.com/cli/).

---

## Virtual kubelet setup

### Cluster and AWS Account

Running ECS Fargate Tasks requires to create at least one ECS cluster and it is currently
recommended to create a dedicated cluster per virtual-kubelet.

```console
aws ecs create-cluster --cluster-name virtual-kubelet-fargate
```

Once the cluster is created, the virtual-kubelet requires permissions to start tasks in that
cluster.

```console
aws iam create-role --role-name virtual-kubelet --assume-role-policy-document '{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "",
      "Effect": "Allow",
      "Principal": {
        "AWS": "MY_ARN"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}'
aws iam attach-role-policy --policy-arn arn:aws:iam::aws:policy/AmazonEC2ContainerServiceFullAccess --role-name virtual-kubelet
aws iam attach-role-policy --policy-arn arn:aws:iam::aws:policy/CloudWatchLogsReadOnlyAccess --role-name virtual-kubelet
```

You can use https://github.com/jtblin/kube2iam to let the virtual-kubelet assume the role, or create an
IAM user and use its secret and key. Replace `MY_ARN` above with the ARN of this role or user.

### Logging

Storing and query container logs is done using AWS CloudWatch Logs and requires
a log group to be created.

```console
aws logs create-log-group --log-group-name /ecs/virtual-kubelet-logs
```

To write logs to the new LogGroup, a role for the ECS executor needs to be created.

```console
aws iam create-role --role-name virtual-kubelet-task-executor --assume-role-policy-document '{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "",
      "Effect": "Allow",
      "Principal": {
        "Service": "ecs-tasks.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}'
aws iam attach-role-policy --policy-arn arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy --role-name virtual-kubelet-task-executor
```

### Networking

For this tutorial we can use the default VPC or any specific VPC in your account.

```console
aws ec2 describe-vpcs --filters Name=isDefault,Values=true --query 'Vpcs[0].VpcId'
aws ec2 describe-subnets --filters "Name=vpc-id,Values=VPC_ID" --query 'Subnets[].SubnetId'
```

All the above obtain values need to be added to the provider config and you can see an example here [example.toml](./example.toml).

### virtual-kubelet Deployment

Configure and deploy the virtual-kubelet to your Kubernetes cluster:

```console
cd providers/aws/examples
vi configmap.yaml           # configure the provider as per example.toml above
kubectl create -R -f .
```

You will also need to request a certificate for the virtual kubelet to allow the Kubernetes API server to request
container logs from it, since this must be done over HTTPS. You can see how to do this in the Kubernetes docs
at https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/#requesting-a-certificate, put the generated certificates
in the secret.yaml file.

## Pod configuration

### Resource limits

To configure the allocatable resources per task you can use the following labels:

| Label | Meaning
|-----------------------------|------------------------|
| `ecs-tasks.amazonaws.com/cpu` | amount of CPU available for the pod |
| `ecs-tasks.amazonaws.com/memory` | amount of memory available for the pod |

By default the `virtual-kubelet` requests `256` CPU units and `512` MiB of memory for per pod.

### IAM role

Similar to kube2iam the `iam.amazonaws.com/role` annotation can be used to assume a
specific IAM role for AWS API calls made by the Kubernetes Pod.
