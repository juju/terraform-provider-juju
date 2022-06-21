# Dependency on the Juju CLI Client Store

## Context and Problem Statement

The provider requires information stored under `~/.local/share/juju` to fulfill some of its functionality  – for example the mapping between a model name and its UUID.

This directory is populated and managed by the Juju command-line tool. It is not possible to obtain some information held here via any of the available APIs.   

In code, the directory is accessed via the filesystem-based client store implementation. There is a memory-based implementation, however, this is only used for testing purposes and cannot be populated from a server-side component.

## Decision

The provider is dependent on the configuration and if not present it will not be able to function. 

This limits the provider to running only where `~/.local/share/juju` is available. The MVP will assume the directory and its configuration always exist and an error will be thrown if it doesn't.    

## Recommendation

Our recommendation is to break this dependency as it impacts the flexibility of Terraform.
