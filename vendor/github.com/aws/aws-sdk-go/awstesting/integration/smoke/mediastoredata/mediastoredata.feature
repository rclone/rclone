# language: en
@mediastoredata @client
Feature: AWS Elemental MediaStore Data Plane

  Scenario: Making a request
    When I call the "ListItems" API
    Then the request should be successful
