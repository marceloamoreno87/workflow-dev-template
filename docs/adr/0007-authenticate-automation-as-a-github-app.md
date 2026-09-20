# Authenticate automation as a GitHub App

The Harness authenticates durable GitHub automation as a private GitHub App installed on the operator's organization, with only the repository and organization permissions required by its workflows. Installation tokens are preferred over a long-lived identity-bearing personal token, while user authorization is reserved for operations that GitHub Projects requires to run on the operator's behalf.
