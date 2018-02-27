extern crate env_logger;
extern crate kube_rust;
#[macro_use]
extern crate log;
extern crate time;
extern crate virtual_kubelet_adapter;

mod utils;
mod unit_provider;

use virtual_kubelet_adapter::start_server;
use unit_provider::UnitProvider;

fn main() {
  // initialize logger
  env_logger::init().unwrap();

  let provider = Box::new(UnitProvider::new());
  start_server(provider).unwrap();
}
