use std::sync::Arc;

use sqlx::postgres::PgPoolOptions;
use tokio::signal;
use tracing::{info, warn, error};
use tracing_subscriber::EnvFilter;

use crypto_engine::config::AppConfig;
use crypto_engine::kafka::consumer::KafkaConsumer;
use crypto_engine::kafka::producer::KafkaProducer;
use crypto_engine::kafka::diploma::DiplomaProcessor;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::from_env("RUST_LOG"))
        .with_target(false)
        .with_thread_ids(true)
        .init();
    
    info!("Starting Crypto Engine service");
    
    let config = AppConfig::load()
        .map_err(|e| anyhow::anyhow!("Failed to load configuration: {}", e))?;
    
    info!(configuration_loaded = true, "Configuration loaded successfully");
    
    let pool = PgPoolOptions::new()
        .max_connections(10)
        .connect(&config.database.url)
        .await
        .map_err(|e| anyhow::anyhow!("Failed to connect to database: {}", e))?;
    
    info!(database_connected = true, "Database connection pool created");
    
    let consumer = KafkaConsumer::new(&config.kafka)
        .map_err(|e| anyhow::anyhow!("Failed to create Kafka consumer: {}", e))?;
    
    let producer = KafkaProducer::new(&config.kafka)
        .map_err(|e| anyhow::anyhow!("Failed to create Kafka producer: {}", e))?;
    
    info!(
        input_topic = %config.kafka.input_topic,
        output_topic = %config.kafka.output_topic,
        "Kafka consumer and producer created"
    );
    
    let config = Arc::new(config);
    let pool = Arc::new(pool);
    let producer = Arc::new(producer);
    
    let (shutdown_tx, mut shutdown_rx) = tokio::sync::broadcast::channel::<()>(1);
    
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
            _ = sigterm.recv() => {
                info!("Received SIGTERM");
            }
        }
        
        let _ = shutdown_tx_clone.send(());
    });
    
    let processor = DiplomaProcessor::new(
        config.clone(),
        pool.clone(),
        producer.clone(),
    );
    
    info!("Diploma processor initialized");
    
    let consumer_task = {
        let processor = processor.clone();
        let _shutdown_rx = shutdown_rx.resubscribe();
        
        tokio::spawn(async move {
            info!("Starting Kafka consumer loop");
            
            let consume_result = consumer.consume(|task| {
                let processor = processor.clone();
                
                async move {
                    processor.process(task).await
                }
            }).await;
            
            match consume_result {
                Ok(()) => info!("Consumer loop completed normally"),
                Err(e) => error!(error = %e, "Consumer loop failed"),
            }
        })
    };
    
    shutdown_rx.recv().await.ok();
    info!("Shutdown signal received, stopping service");
    
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
