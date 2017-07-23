### SDK Features

### SDK Enhancements

### SDK Bugs
* `aws/signer/v4`: Fix out of bounds panic in stripExcessSpaces [#1412](https://github.com/aws/aws-sdk-go/pull/1412)
  * Fixes the out of bands panic in stripExcessSpaces caused by an incorrect calculation of the stripToIdx value. Simplified to code also.
  * Fixes [#1411](https://github.com/aws/aws-sdk-go/issues/1411)
