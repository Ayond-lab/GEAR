# Hack Tooling

This directory holds project-local build support used by the Kubebuilder and controller-gen workflow.

- `boilerplate.go.txt` is passed to `controller-gen object`.
- `../bin/controller-gen` is installed by `make controller-gen` and intentionally ignored by git.

Milestone 1 step 2 converts the dependency-light API skeletons into full Kubernetes runtime objects. At that point `make generate` will produce deepcopy code and `make manifests` will generate strict CRD schemas from kubebuilder markers.

