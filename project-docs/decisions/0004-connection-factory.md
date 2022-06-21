# Add a connection factory to enable model-specific client connections

## Context and Problem Statement

Mutating a model may only be performed using a connection made with its UUID, for example:

```go
connr, err := connector.NewSimple(connector.SimpleConfig{
    ControllerAddresses: "10.43.9.55:17070",
    Username:            "admin",
    Password:            "...",
    CACert:              "...",
    ModelUUID:           "64e1b6ae-b383-404b-ae09-a48585c96112",
})
```

Using a connection with the above configuration we can only modify model `64e1b6ae-b383-404b-ae09-a48585c96112`.

It is not possible to switch the model once a connection is established, you must create a new connection.

The provider creates a single connection without specifying a model, and so we cannot support all model operations.

## Decision

The provider will be refactored to introduce a connection factory that will create connections on-demand. Per-operation a connection will be created with a model UUID and then closed. 

## Concerns

We do not know the impact of opening many short-lived connections on the controller. 

We do not know the impact on provider usability. The connections will scale with the number resources in a Terraform project. This may introduce latency for each operation. 

## Recommendation

Potential concerns may be alleviated by introducing a connection pool, however, we recommend that the connection is not be coupled to a model.