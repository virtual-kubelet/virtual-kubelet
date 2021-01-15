const express = require("express");
const os = require("os");

const app = express();
var bodyParser = require("body-parser");
app.use("/", express.static(__dirname));
app.use(bodyParser.urlencoded({ extended: false }));
app.use(bodyParser.json()); // support json encoded bodies

let demoLogger = (req, res, next) => {
        let current_datetime = new Date();
        let formatted_date =
          current_datetime.getFullYear() +
          "-" +
          (current_datetime.getMonth() + 1) +
          "-" +
          current_datetime.getDate() +
          " " +
          current_datetime.getHours() +
          ":" +
          current_datetime.getMinutes() +
          ":" +
          current_datetime.getSeconds();
        let method = req.method;
        let url = req.url;
        let status = res.statusCode;
        let log = `[${formatted_date}] ${method}:${url} ${status}`;
        console.log(log);
        next();
      };
app.use(demoLogger);

app.post("/", (req, res) => {
  console.log("Ping from : ", req.body.host);
  res.send(`Server: Pong from ${os.hostname()}!`);
});

app.get("/capacity", (req, res) => {
  res.send({ cpu: "8", memory: "5Gi", pods: "20" });
});

app.get("/nodeConditions", (req, res) => {
  res.send([{
    type: "Ready",
    status: "True",
    reason: "KubeletReady",
    message: "kubelet is ready",
    lastHeartbeatTime: (new Date()).toISOString(),
    lastTransitionTime: (new Date()).toISOString(),
  }]);
});

app.get("/nodeAddresses", (req, res) => {
        res.send([{Type: "InternalIP", Address: "127.0.0.1"}]);
});

const port = 3000;
app.listen(port, () => console.log(`listening on port ${port}`));
