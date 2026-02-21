# Development
Clone the source code

```bash
git clone https://github.com/kentrasecurity/kentra.git && cd kentra
```

Build the project with

```bash
make run
```
## Testing
Tests are located in the [test folder](../test) and are called by the [Makefile](../Makefile)
```bash
make test-e2e-kind
```

## Code Style
Before pushing, validate and format the code with

```bash
make fmt
make vet
```

## Remove CRDs and Controller

Uninstall the controller and CRDs:

```bash
make undeploy
make uninstall
```