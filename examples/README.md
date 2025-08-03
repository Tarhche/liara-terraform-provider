# Examples

This directory contains examples that are mostly used for documentation, but can also be run/tested manually via the Terraform CLI.

The document generation tool looks for files in the following locations by default. All other *.tf files besides the ones mentioned below are ignored by the documentation tool. This is useful for creating examples that can run and/or are testable even if some parts are not relevant for the documentation.

* **provider/provider.tf** example file for the provider index page
* **data-sources/`full data source name`/data-source.tf** example file for the named data source page
* **resources/`full resource name`/resource.tf** example file for the named data source page

For testing locally, you need to do the below steps:

1. install the provider using `go install`
2. checking the go bin directory using `echo $GOPATH/bin`
3. creating a provider installation configuration in `~/.terraformrc`

```
provider_installation {

    dev_overrides {
        "registry.terraform.io/tarhche/liara" = "/go/bin"
    }

    # For all other providers, install them directly from their origin provider
    # registries as normal. If you omit this, Terraform will _only_ use
    # the dev_overrides block, and so no other providers will be available.
    direct{}
}
```
