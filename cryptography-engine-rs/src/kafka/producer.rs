use rdkafka::producer::{FutureProducer, FutureRecord};
use rdkafka::config::ClientConfig;
use serde::Serialize;
use std::time::Duration;
use tracing::{info, debug, warn};

use crate::config::KafkaConfig;
use crate::error::{AppError, AppResult};

pub struct KafkaProducer {
    producer: FutureProducer,
    topic: String,
}

impl KafkaProducer {
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
    
    pub async fn send_result(
        &self,
        result: &crate::kafka::messages::ProcessingResult,
    ) -> AppResult<()> {
        let key = if result.diploma_hash.is_empty() {
            format!("{}-{}", result.batch_id, result.record_index)
        } else {
            result.diploma_hash.clone()
        };
        
        self.send(&key, result).await
    }
    
    pub fn topic(&self) -> &str {
        &self.topic
    }
}
