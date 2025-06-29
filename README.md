# Terraform Provider for App Store connect

> _This template repository is built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework). The template repository built on the [Terraform Plugin SDK](https://github.com/hashicorp/terraform-plugin-sdk) can be found at [terraform-provider-scaffolding](https://github.com/hashicorp/terraform-provider-scaffolding). See [Which SDK Should I Use?](https://developer.hashicorp.com/terraform/plugin/framework-benefits) in the Terraform documentation for additional information._

This repository is a [Terraform](https://www.terraform.io) provider for managing resources in [App Store Connect](https://appstoreconnect.apple.com).

This project will be [available on the Terraform Registry](https://registry.terraform.io/providers/Oliver-Binns/appstore/latest).

## Usage

### Provider

Start by declaring an App Store provider.

This should contain:
- Your App Store Connect **Issuer ID**. This is a unique identifier for your App Store Connect API key, which can be found in the [Users and Access](https://appstoreconnect.apple.com/access/api) section of App Store Connect.
- Your App Store Connect **Key ID**. This is the identifier of the API key you generated in App Store Connect.
- Your App Store Connect **Private Key**. This is the contents of the `.p8` private key file you downloaded when creating the API key. You should provide this as a string containing the contents of the file.

```tf
provider "appstoreconnect" {
  issuer_id = "4389f85c-98c6-4023-ab25-8154fcd9460d"
  key_id = "A1234B5678"
  private_key = file("private_api_key.p8")
}
```

### Managing users

You can manage App Store Connect users as a Terraform resource (`appstoreconnect_user`).

Each user requires:
- **first_name**: The user's given name.
- **last_name**: The user's family name or surname.
- **email**: The user's email address, used for login and notifications.
- **all_apps_visible**: Boolean flag specifying if the user has access to all apps in the account.
- **provisioning_allowed**: Boolean flag indicating if the user can access Certificates, Identifiers & Profiles.

```tf
resource "appstoreconnect_user" "example" {
  first_name = "Oliver"
  last_name = "Binns"

  email = "mail@oliverbinns.co.uk"

  all_apps_visible = true
  provisioning_allowed = true
}
```

### App specific permissions

Details coming soon.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the project
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.22

### Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

### Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

### Using the provider

Fill this in for each provider

### Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `make generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```

### Commits

[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) are required for each pull request to ensure that release versioning can be managed automatically.
Please ensure that you have enabled the Git hooks, so that you don't get caught out!:
```
git config core.hooksPath hooks
```