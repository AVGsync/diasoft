//! Crypto Engine - Stateless cryptographic processing worker
//!
//! This service consumes raw diploma tasks from Kafka, performs all cryptographic
//! operations (hashing, signing, encrypting, JWT generation), persists results to
//! PostgreSQL, and publishes processed records back to Kafka.

pub mod config;
pub mod error;
pub mod cryptography;
pub mod db;
pub mod kafka;
pub mod handlers;
