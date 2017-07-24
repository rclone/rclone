# This script tries to build Terraform related packages,
# and find possible breaking changes regarding the Azure
# SDK for Go

set -x

# This should only run on cronjobs
if [ "cron" != $TRAVIS_EVENT_TYPE ]; then
    exit 0
fi

# Only meant to run on latest go version
if [ "go version go1.8 linux/amd64" != "$(go version)" ]; then
    exit 0
fi

go get github.com/kardianos/govendor
REALEXITSTATUS=0

packages=(github.com/hashicorp/terraform
    github.com/terraform-providers/terraform-provider-azurerm
    github.com/terraform-providers/terraform-provider-azure)

for package in ${packages[*]}; do
    go get $package
    cd $GOPATH/src/$package

    # update to latest SDK
    govendor update github.com/Azure/azure-sdk-for-go/...

    # try to build
    make
    REALEXITSTATUS=$(($REALEXITSTATUS+$?))
done

exit $REALEXITSTATUS
