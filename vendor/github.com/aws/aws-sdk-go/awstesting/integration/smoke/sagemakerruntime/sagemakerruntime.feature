# language: en
@sagemakerruntime @client
Feature: Amazon SageMaker Runtime

  Scenario: Making a request
    When I attempt to call the "InvokeEndpoint" API with JSON:
    """
    {"EndpointName": "fake-endpoint", "Body": [123, 125]}
    """
    Then I expect the response error code to be "ValidationError"
