# language: en
@cloudhsmv2 @client
Feature: Amazon CloudHSMv2

  Scenario: Making a request
    When I call the "DescribeBackups" API
    Then the request should be successful
