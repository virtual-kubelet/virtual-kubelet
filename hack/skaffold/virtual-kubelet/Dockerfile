FROM gcr.io/distroless/base

ENV APISERVER_CERT_LOCATION /vkubelet-mock-crt.pem
ENV APISERVER_KEY_LOCATION /vkubelet-mock-key.pem
ENV KUBELET_PORT 10250

# Use the pre-built binary in "bin/virtual-kubelet".
COPY bin/e2e/virtual-kubelet /virtual-kubelet
# Copy the configuration file for the mock provider.
COPY hack/skaffold/virtual-kubelet/vkubelet-mock-cfg.json /vkubelet-mock-cfg.json
# Copy the certificate for the HTTPS server.
COPY hack/skaffold/virtual-kubelet/vkubelet-mock-crt.pem /vkubelet-mock-crt.pem
# Copy the private key for the HTTPS server.
COPY hack/skaffold/virtual-kubelet/vkubelet-mock-key.pem /vkubelet-mock-key.pem

CMD ["/virtual-kubelet"]
