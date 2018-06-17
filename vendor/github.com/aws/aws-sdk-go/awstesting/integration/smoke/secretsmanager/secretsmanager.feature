# language: en
@smoke @secretsmanager
Feature: Amazon SecretsManager

  Scenario: Making a request
    When I call the "ListSecrets" API
    Then the request should be successful

  Scenario: Handling errors
    When I attempt to call the "DescribeSecret" API with:
      | SecretId | fake-secret-id |
    Then I expect the response error code to be "ResourceNotFoundException"

