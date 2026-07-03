---
page_title: "Private testing without the Terraform Registry"
subcategory: ""
description: |-
  How to install and test terraform-provider-matomo from a GitHub Release, before it's published to the Terraform Registry.
---

# Private testing without the Terraform Registry

This provider isn't published to the public Terraform Registry yet. Until it
is, you can still install and test a real, signed release build - the exact
artifact format the registry itself will serve later - using Terraform's
[filesystem mirror](https://developer.hashicorp.com/terraform/cli/config/config-file#filesystem_mirror)
provider installation method.

## 1. Download a release

Go to the repository's [Releases page](https://github.com/nicole-ashley/terraform-provider-matomo/releases)
and download the zip matching your OS and architecture, e.g.
`terraform-provider-matomo_0.1.0_linux_amd64.zip` for 64-bit Linux.

## 2. Lay the binary out for a filesystem mirror

Terraform's filesystem mirror expects a specific directory structure:
`<mirror-root>/registry.terraform.io/nicole-ashley/matomo/<version>/<os>_<arch>/`,
containing the extracted provider binary.

```shell
VERSION=0.1.0
OS_ARCH=linux_amd64  # match the zip you downloaded

mkdir -p ~/.terraform.d/plugins/registry.terraform.io/nicole-ashley/matomo/${VERSION}/${OS_ARCH}
unzip terraform-provider-matomo_${VERSION}_${OS_ARCH}.zip -d ~/.terraform.d/plugins/registry.terraform.io/nicole-ashley/matomo/${VERSION}/${OS_ARCH}
```

## 3. Point Terraform at the mirror

Add this to `~/.terraformrc` (create the file if it doesn't exist):

```hcl
provider_installation {
  filesystem_mirror {
    path    = "/home/you/.terraform.d/plugins"
    include = ["registry.terraform.io/nicole-ashley/matomo"]
  }
  direct {
    exclude = ["registry.terraform.io/nicole-ashley/matomo"]
  }
}
```

Replace `/home/you/.terraform.d/plugins` with the real, absolute path to the
directory from step 2 (not `~` - Terraform's CLI config does not expand it).

## 4. Use it like any registry provider

```hcl
terraform {
  required_providers {
    matomo = {
      source  = "nicole-ashley/matomo"
      version = "0.1.0"
    }
  }
}

provider "matomo" {
  base_url  = "https://analytics.example.com"
  api_token = var.matomo_api_token
}
```

Run `terraform init`. Terraform will read the binary straight from the
mirror instead of contacting the registry, while verifying it against the
release's checksums exactly as it would for a registry-hosted provider.
There is no `dev_overrides` block and no special provider source syntax
here - the configuration above is identical to what you'd write once the
provider is actually published, so nothing needs to change later.
