@upload
Feature: the upload feature

  Scenario: create the uploader
    When initialize uploader
    Then uploader is initialized

  Scenario: upload large file
    When upload a large file
    Then the large file is uploaded
