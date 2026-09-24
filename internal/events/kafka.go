package events

import (
	"context"
	"encoding/json"

	"github.com/IBM/sarama"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/domain"
)

type Publisher interface {
	PublishTransaction(context.Context, domain.Transaction) error
}

type KafkaPublisher struct {
	producer sarama.SyncProducer
	topic    string
}

func NewKafkaPublisher(brokers []string, topic string) (*KafkaPublisher, error) {
	c := sarama.NewConfig()
	c.Version = sarama.V3_6_0_0
	c.Producer.Return.Successes = true
	c.Producer.RequiredAcks = sarama.WaitForAll
	p, err := sarama.NewSyncProducer(brokers, c)
	if err != nil {
		return nil, err
	}
	return &KafkaPublisher{producer: p, topic: topic}, nil
}
func (p *KafkaPublisher) Close() error { return p.producer.Close() }
func (p *KafkaPublisher) PublishTransaction(ctx context.Context, t domain.Transaction) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	m := &sarama.ProducerMessage{Topic: p.topic, Key: sarama.StringEncoder(t.AccountID), Value: sarama.ByteEncoder(b), Headers: []sarama.RecordHeader{{Key: []byte("event-type"), Value: []byte("transaction.ingested.v1")}}}
	_, _, err = p.producer.SendMessage(m)
	return err
}

type Consumer struct {
	group sarama.ConsumerGroup
	topic string
}

func NewConsumer(brokers []string, topic, groupID string) (*Consumer, error) {
	c := sarama.NewConfig()
	c.Version = sarama.V3_6_0_0
	c.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	c.Consumer.Offsets.Initial = sarama.OffsetOldest
	g, err := sarama.NewConsumerGroup(brokers, groupID, c)
	if err != nil {
		return nil, err
	}
	return &Consumer{group: g, topic: topic}, nil
}
func (c *Consumer) Close() error { return c.group.Close() }
func (c *Consumer) Run(ctx context.Context, fn func(context.Context, domain.Transaction) error) error {
	h := handler{fn: fn}
	for ctx.Err() == nil {
		if err := c.group.Consume(ctx, []string{c.topic}, h); err != nil {
			return err
		}
	}
	return ctx.Err()
}

type handler struct {
	fn func(context.Context, domain.Transaction) error
}

func (handler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (handler) Cleanup(sarama.ConsumerGroupSession) error { return nil }
func (h handler) ConsumeClaim(s sarama.ConsumerGroupSession, cl sarama.ConsumerGroupClaim) error {
	for m := range cl.Messages() {
		var t domain.Transaction
		if err := json.Unmarshal(m.Value, &t); err != nil {
			continue
		}
		if err := h.fn(s.Context(), t); err != nil {
			return err
		}
		s.MarkMessage(m, "")
	}
	return nil
}
