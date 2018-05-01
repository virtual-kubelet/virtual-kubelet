# Comparison between vSPC and CAF for serial communication
This document provides a comparison between two options that were considered to handle vMotion during an active connection between a user and a containerVM's serial port. The two options investigated are:
- Virtual Serial Port Concentrator
- Common Agent Framework

In the first section, a very brief description of both solutions is provided. In the second section, a comparison between the capabilities of the two solutions is discussed. Finally, a solution is proposed with detailed action items.

# First: Introduction
### Virtual Serial Port Concentrator (vSPC)
vSPC is a telnet server proxy that aggregates and multiplexes serial port communication from multiple VMs to a remote system. vSPC allows vMotion to continue even if there exists an active connection between a remote system and the serial port of a VM.

For vSPC to work, the telnet server proxy must support the following:

1. VMware's telnet exetension commands
2. Handle extension commands for vMotion notification
3. forward connections to a remote system

A more in-depth description of this solution is very well documented in https://www.vmware.com/support/developer/vc-sdk/visdk41pubs/vsp41_usingproxy_virtual_serial_ports.pdf

### Common Agent Framework (CAF)
Multiple VMware's management solutions (e.d Loginsight) require the installation of guest agents. Even though multiple agents have common functionalities, this commonality is not efficiently utilized.  

CAF attempts to solve many of these problems by providing a common framework that simplifies standard deployment/upgrade mechanism, common scalable communication layers, common in-guest authorization, common provider frameworks, common data representation(s), etc.

CAF provides a secure, scalable, high-bandwidth message bus that is currently implemented via a message broker proxy that talks to a rabbitmq node (or cluster)

Serial communication from containerVMs to the VIC appliance can leverage the CAF to provide a high-bandwidth and secure communication channel.

# Second: Comparison
Comparison Metric | vSPC | CAF
------------------|------|------
Ease of implementation/integration|relatively easy|more challenging<sup>1</sup>
Communication Bandwidth|relatively small|relatively large
Connection Persistence during vMotion|Supported|Supported
No ESX host agent| Not required | Not required<sup>2</sup>
No Network stack required for containerVMs|None required|None required<sup>3</sup>

<sup>1</sup>CAF will require a lot of across-team communication and a major code refactoring

<sup>2</sup>Even though, AFAIK, there is no ESX host agent requirement, there is an extra overhead associated with refactoring the code to utilize the CAF API. Furthermore, the `rabbitmqproxy` in the ESX host should be configured to be able to communicate with the broker

<sup>3</sup>The Rabbitmq node (or cluster) has to be connected to the management network

# Third: Proposal
Based on the priority criteria put forward in issue #3937, vSPC seems to be the right way to go because of its ease of implememntation/integration with VIC's code base. The only significant difference between using CAF and vSPC is the high-bandwidth communication we are projected to get should we use CAF. Until performance becomes an issue, vSPC seems to be a very good candidate.
Adopting the vSPC solution will require the following:

1. Implementing an extensible telnet server
2. Implementing a vSPC
3. integrating the vSPC with VIC

Step 1 is required because, AFAIK, the current go telnet implementations do not support extending the telnet commands which is a requirement on our end. The output of this step will be a stand-alone library

Step 2 is required because, AFAIK, the only vSPC implementation I have seen is written in python. The output of this step will be a stand-alone binary
