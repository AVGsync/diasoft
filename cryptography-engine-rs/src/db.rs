//! Database interaction module.
//!
//! All PostgreSQL interaction using `sqlx` with compile-time `query_as!` macros.
//! Run `cargo sqlx prepare` before committing to regenerate the offline query cache.

pub mod models;
pub mod repository;
