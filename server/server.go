package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	logcanal_proto "enge-go-sidecar-grpc-log/proto"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.WarnLevel)
}

var MESSAGE_COUNT = 0
var TOPIC string = "log-servico-canal"
var KAFKA_BOOTSTRAP_SERVERS = os.Getenv("BOOTSTRAP_SERVERS")

func report(deliveryChan chan bool, k *kafka.Producer) {
	defer close(deliveryChan)

	for e := range k.Events() {
		switch ev := e.(type) {
		case *kafka.Message:
			m := ev
			if m.TopicPartition.Error != nil {
				log.Error("Delivery failed: %v\n", m.TopicPartition.Error)
			} else {
				log.Debug("Message delivered.")
			}
			return
		default:
			fmt.Printf("Ignored event: %s\n", ev)
		}
	}
}

type logCanalServiceServer struct {
	logcanal_proto.UnimplementedLogCanalServiceServer
}

func (s *logCanalServiceServer) Sink(stream logcanal_proto.LogCanalService_SinkServer) error {
	k, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":            KAFKA_BOOTSTRAP_SERVERS,
		"go.delivery.reports":          true,
		"queue.buffering.max.ms":       50,
		"retries":                      20,
		"queue.buffering.max.messages": 100000,
	})
	if err != nil {
		panic(err)
	}

	deliveryChan := make(chan bool)
	go report(deliveryChan, k)

	for {
		MessageRequest, err := stream.Recv()
		MESSAGE_COUNT += 1
		if err == io.EOF {
			println("Total messages: ", MESSAGE_COUNT)
			resp := &logcanal_proto.ResponseStatus{FallBackEnabled: 0}
			return stream.SendAndClose(resp)
		}
		if err != nil {
			return err
		}
		message, _ := protojson.Marshal(MessageRequest)
		k.ProduceChannel() <- &kafka.Message{TopicPartition: kafka.TopicPartition{Topic: &TOPIC, Partition: kafka.PartitionAny}, Value: []byte(message)}
	}
	_ = <-deliveryChan
	k.Close()
	return nil
}

func newServer() *logCanalServiceServer {
	s := &logCanalServiceServer{}
	return s
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", "0.0.0.0:50001")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	logcanal_proto.RegisterLogCanalServiceServer(grpcServer, newServer())
	grpcServer.Serve(lis)
}
