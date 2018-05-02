# language: en
@autoscalingplans @client
Feature: AWS Auto Scaling Plans

  Scenario: Making a request
    When I call the "DescribeScalingPlans" API
    Then the request should be successful
