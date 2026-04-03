//! Kafka producer wrapper for `diplomas.processing_results` topic.
//!
//! Wraps `rdkafka::FutureProducer`. Exposes a generic `send` method.
//! Uses `diploma_hash` as the Kafka message key for consistent partition routing.

use rdkafka::producer::{FutureProducer, FutureRecord};
use rdkafka::config::ClientConfig;
use serde::Serialize;
use std::time::Duration;
use tracing::{info, debug, warn};

use crate::config::KafkaConfig;
use crate::error::{AppError, AppResult};

/// Kafka producer wrapper for processing results
pub struct KafkaProducer {
    producer: FutureProducer,
    topic: String,
}

impl KafkaProducer {
    /// Creates a new Kafka producer from configuration
    pub fn new(config: &KafkaConfig) -> AppResult<Self> {
        let producer: FutureProducer = ClientConfig::new()
            .set("bootstrap.servers", &config.brokers)
            .set("message.timeout.ms", "5000")
            .set("compression.type", "snappy")
            .create()?;
        
        let topic = config.output_topic.clone();
        
        info!("Kafka producer created for topic: {}", topic);
        
        Ok(Self { producer, topic })
    }
    
    /// Sends a message to the output topic.
    ///
    /// Uses the provided key for partition routing. For diploma processing
    /// results, the `diploma_hash` should be used as the key to ensure
    /// consistent ordering.
    ///
    /// # Arguments
    /// * `key` - Message key (used for partition routing)
    /// * `payload` - Serializable payload to send as JSON
    pub async fn send<T: Serialize>(
        &self,
        key: &str,
        payload: &T,
    ) -> AppResult<()> {
        let payload_json = serde_json::to_string(payload)?;
        
        debug!(
            topic = %self.topic,
            key = %key,
            payload_size = payload_json.len(),
            "Sending message to Kafka"
        );
        
        let delivery_result = self.producer
            .send(
                FutureRecord::to(&self.topic)
                    .key(key)
                    .payload(&payload_json),
                Duration::from_secs(5),
            )
            .await;
        
        match delivery_result {
            Ok((partition, offset)) => {
                debug!(
                    topic = %self.topic,
                    partition = partition,
                    offset = offset,
                    "Message delivered successfully"
                );
                Ok(())
            }
            Err((e, _)) => {
                warn!("Failed to send message: {}", e);
                Err(AppError::Kafka(e))
            }
        }
    }
    
    /// Sends a processing result to the output topic.
    ///
    /// Uses `diploma_hash` as the message key for consistent partition routing.
    ///
    /// # Arguments
    /// * `result` - Processing result to send
    pub async fn send_result(
        &self,
        result: &crate::kafka::messages::ProcessingResult,
    ) -> AppResult<()> {
        let key = if result.diploma_hash.is_empty() {
            // For error results without a hash, use batch_id + index as key
            format!("{}-{}", result.batch_id, result.record_index)
        } else {
            result.diploma_hash.clone()
        };
        
        self.send(&key, result).await
    }
    
    /// Returns the topic name this producer sends to
    pub fn topic(&self) -> &str {
        &self.topic
    }
}
