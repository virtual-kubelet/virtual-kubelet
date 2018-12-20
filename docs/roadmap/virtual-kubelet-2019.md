# 2019 Virtual Kubelet roadmap

**work in progress**

## Core ideals for next year

1. Reduce the barrier of entry.
2. Create a stable community.
3. Release virtual kubelet 1.0.

### Requirements and goals

1. **Core.** Split out providers from the virtual kubelet core tree. Today the provider dependencies within the virtual kubelet cause more harm than good.
2. **Interface.** Stablize the virtual kubelet interface so minimal changes to no changes will be needed in the future. 
3. **Developer.** Reduce the barrier of entry to create with virtual kubelet.
4. **Community.** Grow and nurture the community for virtual kubelet core. Includes compiling use-cases past the cloud providers interests. IoT use-cases will be a big focus. 

### Testing

1. Improve our e2e testing in virtual kubelet core.
2. Add options & integration points to include any provider.
3. Create a baseline for how *quickly* virtual kubelet can process requests. Test at high scale throughput. 
4. Work with sig-architecture in Kubernetes to develop a conformance test profile for virtual kubelet. 

### Use cases 

1. Explore what it means to use virtual kubelet in an IoT Edge usecases and work with wg-io-edge in Kubernetes to develop a standard within virtual kubelet to enable those usecases.
2. Create provider agnostic tools to scale into virtual kubelet.
3. tba