# kwir - deployment

Two kustomize bases are provided.

These bases only differs by how they deal with certificates & secrets :

* `kwir-manual-certs` : Webhook's TLS secrets & cluster CA bundle are not specified and must be generated & managed by the cluster admin.

* `kwir-cert-managed` : Webhook's TLS secrets are managed by an existing cert-manager instance. Self-signed certs are automatically generated and stored as secret, CA bundle is also injected in kwir webhook specs by ca-injector.

We highly recommend using the cert-manager option, which reduce massively the certificate management overhead !
