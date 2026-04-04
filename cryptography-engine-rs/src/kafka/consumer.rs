use rdkafka::consumer::{Consumer, StreamConsumer};
use rdkafka::message::Message;
use rdkafka::config::ClientConfig;
use tokio_stream::StreamExt;
use tracing::{info, warn, error, debug};

use crate::config::KafkaConfig;
use crate::error::{AppResult};
use crate::kafka::messages::DiplomaTask;

pub struct KafkaConsumer {
    consumer: StreamConsumer,
    topic: String,
}

impl KafkaConsumer {
    pub fn new(config: &KafkaConfig) -> AppResult<Self> {
        let consumer: StreamConsumer = ClientConfig::new()
            .set("bootstrap.servers", &config.brokers)
            .set("group.id", &config.group_id)
            .set("auto.offset.reset", "earliest")
            .set("enable.auto.commit", "false")
            .set("session.timeout.ms", "6000")
            .set("max.poll.interval.ms", "300000")
            .create()?;
        
        let topic = config.input_topic.clone();
        
        consumer.subscribe(&[&topic])?;
        info!("Kafka consumer subscribed to topic: {}", topic);
        
        Ok(Self { consumer, topic })
    }
    
    pub async fn consume<F, Fut>(
        &self,
        mut handler: F,
    ) -> AppResult<()>
    where
        F: FnMut(DiplomaTask) -> Fut + Send,
        Fut: std::future::Future<Output = AppResult<()>> + Send,
    {
        let mut stream = self.consumer.stream();
        
        while let Some(message_result) = stream.next().await {
            match message_result {
                Ok(borrowed_message) => {
                    let payload = borrowed_message.payload();
                    
                    if let Some(payload_bytes) = payload {
                        match serde_json::from_slice::<DiplomaTask>(payload_bytes) {
                            Ok(task) => {
                                debug!(
                                    batch_id = %task.batch_id,
                                    record_index = task.record_index,
                                    "Received diploma task"
                                );
                                
                                match handler(task).await {
                                    Ok(()) => {
                                        if let Err(e) = self.consumer.commit_message(&borrowed_message, rdkafka::consumer::CommitMode::Async) {
                                            warn!("Failed to commit offset: {}", e);
                                        }
                                    }
                                    Err(e) => {
                                        error!("Handler error (message will be redelivered): {}", e);
                                    }
                                }
                            }
                            Err(e) => {
                                error!("Failed to deserialize DiplomaTask: {}", e);
                                let _ = self.consumer.commit_message(&borrowed_message, rdkafka::consumer::CommitMode::Async);
                            }
                        }
                    } else {
                        warn!("Received message with empty payload");
                    }
                }
                Err(e) => {
                    warn!("Kafka consumer error: {}", e);
                    tokio::time::sleep(tokio::time::Duration::from_millis(100)).await;
                }
            }
        }
        
        Ok(())
    }
    
    pub fn topic(&self) -> &str {
        &self.topic
    }
}
