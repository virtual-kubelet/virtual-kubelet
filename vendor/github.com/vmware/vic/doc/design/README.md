# vSphere Integrated Containers - Architecture and Design

The bulk of the design notes currently relate to [components and their role in the system](components.md) - high level descriptions primarily.

Networking is broken out into a subfolder on the expectation that it will become a significantly larger area once NSX integration is addressed - [the 1.0 MVP network design](networking/README.md) does not encompass NSX.


Documentation about component interactions is ongoing with the initial docs being:
* [configuration](configuration.md)
* [communication between VCH appliance and containerVMs](communications.md)
* portlayer component communication
* security
* [installation and self-provisioning - usage examples](../user/usage.md)
* [installation and self-provisioning - technical](vic-machine.md)
