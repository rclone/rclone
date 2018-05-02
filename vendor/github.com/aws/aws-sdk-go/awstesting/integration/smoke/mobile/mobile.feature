# language: en
@mobile @client
Feature: AWS Mobile

  Scenario: Making a request
    When I call the "ListBundles" API
    Then the request should be successful
