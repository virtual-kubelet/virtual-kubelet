# Changelog

## 0.5.6 (2016-01-14)
- Setup embedded DNS server correctly on container restart. Fixes docker/docker#19354

## 0.5.5 (2016-01-14)
- Allow network-scoped alias to be resolved for anonymous endpoint
- Self repair corrupted IP database that could happen in 1.9.0 & 1.9.1
- Skip IPTables cleanup if --iptables=false is set. Fixes docker/docker#19063

## 0.5.4 (2016-01-12)
- Removed the isNodeAlive protection when user forces an endpoint delete

## 0.5.3 (2016-01-12)
- Bridge driver supporting internal network option
- Backend implementation to support "force" option to network disconnect
- Fixing a regex in etchosts package to fix docker/docker#19080

## 0.5.2 (2016-01-08)
- Embedded DNS replacing /etc/hosts based Service Discovery
- Container local alias and Network-scoped alias support
- Backend support for internal network mode
- Support for IPAM driver options
- Fixes overlay veth cleanup issue : docker/docker#18814
- fixes docker/docker#19139
- disable IPv6 Duplicate Address Detection

## 0.5.1 (2015-12-07)
- Allowing user to assign IP Address for containers
- Fixes docker/docker#18214
- Fixes docker/docker#18380

## 0.5.0 (2015-10-30)

- Docker multi-host networking exiting experimental channel
- Introduced IP Address Management and IPAM drivers
- DEPRECATE service discovery from default bridge network
- Introduced new network UX
- Support for multiple networks in bridge driver
- Local persistance with boltdb

## 0.4.0 (2015-07-24)

- Introduce experimental version of Overlay driver
- Introduce experimental version of network plugins
- Introduce experimental version of network & service UX
- Introduced experimental /etc/hosts based service discovery
- Integrated with libkv
- Improving test coverage
- Fixed a bunch of issues with osl namespace mgmt

## 0.3.0 (2015-05-27)
 
- Introduce CNM (Container Networking Model)
- Replace docker networking with CNM & Bridge driver
