use kube_rust::models::{V1NodeAddress, V1NodeCondition, V1NodeDaemonEndpoints, V1Pod, V1PodStatus};
use virtual_kubelet_adapter::{Error, Provider, Result};
use std::collections::BTreeMap;
use utils::Filter;
use time;

pub struct UnitProvider {
    pods_map: BTreeMap<String, V1Pod>,
}

impl UnitProvider {
    pub fn new() -> UnitProvider {
        UnitProvider {
            pods_map: BTreeMap::new(),
        }
    }

    fn make_pod_id(&self, pod: &V1Pod) -> String {
        let empty = String::from("");
        pod.metadata()
            .map(|m| m.name().unwrap_or(&empty))
            .unwrap_or(&empty)
            .clone()
    }

    fn pod_name(&mut self, pod: &V1Pod) -> String {
        let empty = "".to_owned();
        format!(
            "{}",
            pod.metadata()
                .map(|m| m.name())
                .and_then(|s| s)
                .unwrap_or(&empty)
        )
    }
}

impl Provider for UnitProvider {
    fn create_pod(&mut self, pod: &V1Pod) -> Result<()> {
        info!("Creating pod: {}", self.pod_name(pod));

        let id = self.make_pod_id(pod);
        let mut new_pod = pod.clone();
        new_pod.set_status(
            pod.status()
                .map(|s| s.clone())
                .unwrap_or_else(|| V1PodStatus::new())
                .with_phase(String::from("Running")),
        );
        self.pods_map.insert(id, new_pod);
        Ok(())
    }

    fn update_pod(&mut self, pod: &V1Pod) -> Result<()> {
        info!("Updating pod: {}", self.pod_name(pod));

        // update the pod definition if it exists
        let id = self.make_pod_id(pod);
        if self.pods_map.contains_key(&id) {
            self.pods_map.insert(id, pod.clone());
        }
        Ok(())
    }

    fn delete_pod(&mut self, pod: &V1Pod) -> Result<()> {
        info!("Deleting pod: {}", self.pod_name(pod));

        let id = self.make_pod_id(pod);
        self.pods_map.remove(&id);
        Ok(())
    }

    fn get_pod(&self, namespace: &str, name: &str) -> Result<V1Pod> {
        info!("Getting pod: {}", name);
        self.pods_map
            .get(name)
            .xfilter(|pod| {
                let empty = String::from("");
                let ns = pod.metadata()
                    .map(|m| m.namespace())
                    .and_then(|n| n)
                    .unwrap_or(&empty);
                namespace.len() == 0 || namespace == ns
            })
            .map(|pod| pod.clone())
            .ok_or_else(|| {
                info!("Could not find pod: {}", name);
                Error::not_found("Pod not found")
            })
    }

    fn get_container_logs(
        &self,
        namespace: &str,
        pod_name: &str,
        container_name: &str,
        tail: i32,
    ) -> Result<String> {
        Ok(format!(
            "get_container_logs() - ns: {}, pod_name: {}, container_name: {}, tail: {}",
            namespace, pod_name, container_name, tail
        ))
    }

    fn get_pod_status(&self, _: &str, name: &str) -> Result<V1PodStatus> {
        info!("Getting pod status: {}", name);
        self.pods_map
            .get(name)
            .map(|pod| pod.status())
            .and_then(|pod_status| pod_status)
            .map(|pod_status| pod_status.clone())
            .ok_or_else(|| {
                info!("Could not find pod/status: {}", name);
                Error::not_found("Pod/status not found")
            })
    }

    fn get_pods(&self) -> Result<Vec<V1Pod>> {
        info!("Getting pods");
        Ok(self.pods_map.values().cloned().collect())
    }

    fn capacity(&self) -> Result<BTreeMap<String, String>> {
        info!("Getting capacity");
        let values = [("cpu", "20"), ("memory", "100Gi"), ("pods", "20")];
        let mut map = BTreeMap::new();
        for v in values.iter() {
            map.insert(v.0.to_string(), v.1.to_string());
        }

        Ok(map)
    }

    fn node_conditions(&self) -> Result<Vec<V1NodeCondition>> {
        info!("Getting node_condition");
        Ok(vec![
            V1NodeCondition::new(String::from("True"), String::from("Ready"))
                .with_reason(String::from("KubeletReady"))
                .with_message(String::from("Rusty times."))
                .with_last_heartbeat_time(format!("{}", time::now_utc().rfc3339()))
                .with_last_transition_time(format!("{}", time::now_utc().rfc3339())),
        ])
    }

    fn node_addresses(&self) -> Result<Vec<V1NodeAddress>> {
        Ok(vec![])
    }

    fn node_daemon_endpoints(&self) -> Result<V1NodeDaemonEndpoints> {
        Err(Error::new("Not implemented"))
    }

    fn operating_system(&self) -> String {
        String::from("linux")
    }
}
