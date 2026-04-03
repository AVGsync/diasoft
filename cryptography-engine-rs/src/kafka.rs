//! Kafka consumer and producer wrappers.
//!
//! Thin wrappers over `rdkafka`. Zero business logic — only transport concerns.

pub mod messages;
pub mod consumer;
pub mod producer;
