package pkg

import (
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// InitKafkaWriter 创建并返回一个 Kafka Writer
// 你可以改用从 os.Getenv() 读取 broker/topic
func InitKafkaWriter(broker string) *kafka.Writer {
	w := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{broker},
		Balancer: &kafka.LeastBytes{},

		// 以下参数可按需调优
		Async:            false,
		QueueCapacity:    1000,
		BatchSize:        1,
		BatchTimeout:     10 * time.Millisecond,
		RequiredAcks:     -1,
		CompressionCodec: kafka.Snappy.Codec(),
	})
	log.Printf("Kafka Writer init success: broker=%s, topic will assign when write\n", broker)
	return w
}
