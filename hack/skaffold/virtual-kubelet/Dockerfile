FROM gcr.io/distroless/base

ENV APISERVER_CERT_LOCATION /vkubelet-mock-0-crt.pem
ENV APISERVER_KEY_LOCATION /vkubelet-mock-0-key.pem
ENV KUBELET_PORT 10250

# Use the pre-built binary in "bin/virtual-kubelet".
COPY bin/e2e/virtual-kubelet /virtual-kubelet
# Copy the configuration file for the mock provider.
COPY hack/skaffold/virtual-kubelet/vkubelet-mock-0-cfg.json /vkubelet-mock-0-cfg.json
# Copy the certificate for the HTTPS server.
COPY hack/skaffold/virtual-kubelet/vkubelet-mock-0-crt.pem /vkubelet-mock-0-crt.pem
# Copy the private key for the HTTPS server.
COPY hack/skaffold/virtual-kubelet/vkubelet-mock-0-key.pem /vkubelet-mock-0-key.pem

CMD ["/virtual-kubelet"]
