# language: en
@mediastore @client
Feature: AWS Elemental MediaStore

  Scenario: Making a request
    When I call the "ListContainers" API
    Then the request should be successful
