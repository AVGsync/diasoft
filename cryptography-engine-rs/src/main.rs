//! Crypto Engine - Stateless cryptographic processing worker
//!
//! This service consumes raw diploma tasks from Kafka, performs all cryptographic
//! operations (hashing, signing, encrypting, JWT generation), persists results to
//! PostgreSQL, and publishes processed records back to Kafka.
//!
//! It never exposes an HTTP API — all I/O is Kafka + DB.

use std::sync::Arc;

use sqlx::postgres::PgPoolOptions;
use tokio::signal;
use tracing::{info, warn, error};
use tracing_subscriber::EnvFilter;

use crypto_engine::config::AppConfig;
use crypto_engine::db::repository::is_batch_complete;
use crypto_engine::handlers::diploma::process;
use crypto_engine::kafka::consumer::KafkaConsumer;
use crypto_engine::kafka::producer::KafkaProducer;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    // Initialize tracing subscriber
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::from_env("RUST_LOG"))
        .with_target(false)
        .with_thread_ids(true)
        .init();
    
    info!("Starting Crypto Engine service");
    
    // Load configuration
    let config = AppConfig::load()
        .map_err(|e| anyhow::anyhow!("Failed to load configuration: {}", e))?;
    
    info!(configuration_loaded = true, "Configuration loaded successfully");
    
    // Create database connection pool
    let pool = PgPoolOptions::new()
        .max_connections(10)
        .connect(&config.database.url)
        .await
        .map_err(|e| anyhow::anyhow!("Failed to connect to database: {}", e))?;
    
    info!(database_connected = true, "Database connection pool created");
    
    // Create Kafka consumer and producer
    let consumer = KafkaConsumer::new(&config.kafka)
        .map_err(|e| anyhow::anyhow!("Failed to create Kafka consumer: {}", e))?;
    
    let producer = KafkaProducer::new(&config.kafka)
        .map_err(|e| anyhow::anyhow!("Failed to create Kafka producer: {}", e))?;
    
    info!(
        input_topic = %config.kafka.input_topic,
        output_topic = %config.kafka.output_topic,
        "Kafka consumer and producer created"
    );
    
    // Wrap in Arc for sharing across tasks
    let config = Arc::new(config);
    let pool = Arc::new(pool);
    let producer = Arc::new(producer);
    
    // Set up graceful shutdown
    let (shutdown_tx, mut shutdown_rx) = tokio::sync::broadcast::channel::<()>(1);
    
    // Handle SIGTERM and SIGINT
    let shutdown_tx_clone = shutdown_tx.clone();
    tokio::spawn(async move {
        let ctrl_c = signal::ctrl_c();
        #[cfg(unix)]
        let mut sigterm = signal::unix::signal(signal::unix::SignalKind::terminate())
            .expect("Failed to install SIGTERM handler");
        #[cfg(not(unix))]
        let sigterm = std::future::pending::<()>();
        
        tokio::select! {
            _ = ctrl_c => {
                info!("Received SIGINT (Ctrl+C)");
            }
            _ = sigterm => {
                info!("Received SIGTERM");
            }
        }
        
        let _ = shutdown_tx_clone.send(());
    });
    
    // Spawn the consumer loop
    let consumer_task = {
        let config = config.clone();
        let pool = pool.clone();
        let producer = producer.clone();
        let mut shutdown_rx = shutdown_rx.resubscribe();
        
        tokio::spawn(async move {
            info!("Starting Kafka consumer loop");
            
            let consume_result = consumer.consume(|task| {
                let config = config.clone();
                let pool = pool.clone();
                let producer = producer.clone();
                
                async move {
                    process(task, &config, &pool, &producer).await;
                    Ok(())
                }
            }).await;
            
            match consume_result {
                Ok(()) => info!("Consumer loop completed normally"),
                Err(e) => error!(error = %e, "Consumer loop failed"),
            }
        })
    };
    
    // Wait for shutdown signal
    shutdown_rx.recv().await.ok();
    info!("Shutdown signal received, stopping service");
    
    // Wait for consumer task to finish (with timeout)
    match tokio::time::timeout(
        std::time::Duration::from_secs(10),
        consumer_task
    ).await {
        Ok(Ok(())) => info!("Consumer task stopped gracefully"),
        Ok(Err(e)) => warn!(error = %e, "Consumer task exited with error"),
        Err(_) => warn!("Consumer task did not stop within timeout"),
    }
    
    info!("Crypto Engine service stopped");
    Ok(())
}
