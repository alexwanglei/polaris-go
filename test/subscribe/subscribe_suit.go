/**
 * Tencent is pleased to support the open source community by making polaris-go available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package subscribe

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/uuid"
	apimodel "github.com/polarismesh/specification/source/go/api/v1/model"
	"github.com/polarismesh/specification/source/go/api/v1/service_manage"
	"github.com/polarismesh/specification/source/go/api/v1/traffic_manage"
	"google.golang.org/grpc"
	"gopkg.in/check.v1"

	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/config"
	"github.com/polarismesh/polaris-go/pkg/model"
	"github.com/polarismesh/polaris-go/test/mock"
	"github.com/polarismesh/polaris-go/test/util"
)

const (
	// 测试的默认命名空间
	consumerNamespace = "testns"
	// 测试的默认服务名
	consumerService = "svc1"
	// 测试服务器的默认地址
	consumerIPAddress = "127.0.0.1"
	// 测试服务器的端口
	consumerPort = 8008
)

const (
	// 直接过滤的实例数
	normalInstances    = 3
	isolatedInstances  = 2
	unhealthyInstances = 1
	allInstances       = normalInstances + isolatedInstances + unhealthyInstances
)

// EventSubscribeSuit 限流相关的用例集
type EventSubscribeSuit struct {
	mockServer   mock.NamingServer
	grpcServer   *grpc.Server
	grpcListener net.Listener
	serviceToken string
	testService  *service_manage.Service
}

func (t *EventSubscribeSuit) addInstance() []*service_manage.Instance {
	return t.mockServer.GenTestInstancesWithHostPort(t.testService, 1, consumerIPAddress, 2000)
}

// SetUpSuite 初始化测试套件
func (t *EventSubscribeSuit) SetUpSuite(c *check.C) {
	grpcOptions := make([]grpc.ServerOption, 0)
	maxStreams := 100000
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(uint32(maxStreams)))

	// get the grpc server wired up
	grpc.EnableTracing = true

	ipAddr := consumerIPAddress
	shopPort := consumerPort
	var err error
	t.grpcServer = grpc.NewServer(grpcOptions...)
	t.serviceToken = uuid.New().String()
	t.mockServer = mock.NewNamingServer()
	token := t.mockServer.RegisterServerService(config.ServerDiscoverService)
	t.mockServer.RegisterServerInstance(ipAddr, shopPort, config.ServerDiscoverService, token, true)
	t.mockServer.RegisterNamespace(&apimodel.Namespace{
		Name:    &wrappers.StringValue{Value: consumerNamespace},
		Comment: &wrappers.StringValue{Value: "for consumer api test"},
		Owners:  &wrappers.StringValue{Value: "ConsumerAPI"},
	})
	t.mockServer.RegisterServerServices(ipAddr, shopPort)
	t.testService = &service_manage.Service{
		Name:      &wrappers.StringValue{Value: consumerService},
		Namespace: &wrappers.StringValue{Value: consumerNamespace},
		Token:     &wrappers.StringValue{Value: t.serviceToken},
	}
	t.mockServer.RegisterService(t.testService)
	t.mockServer.GenTestInstances(t.testService, normalInstances)

	service_manage.RegisterPolarisGRPCServer(t.grpcServer, t.mockServer)
	t.grpcListener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", ipAddr, shopPort))
	if err != nil {
		log.Fatal(fmt.Sprintf("error listening appserver %v", err))
	}
	log.Printf("appserver listening on %s:%d\n", ipAddr, shopPort)
	go func() {
		t.grpcServer.Serve(t.grpcListener)
	}()
}

// TearDownSuite 结束测试套程序
func (t *EventSubscribeSuit) TearDownSuite(c *check.C) {
	t.grpcServer.Stop()
	if util.DirExist(util.BackupDir) {
		os.RemoveAll(util.BackupDir)
	}
}

// GetInstanceEvent 获取实例事件
func (t *EventSubscribeSuit) GetInstanceEvent(ch <-chan model.SubScribeEvent) (model.SubScribeEvent, error) {
	select {
	case e := <-ch:
		return e, nil
	default:
		return nil, nil
	}
}

// // TestInstanceEvent 测试实例事件
// func (t *EventSubscribeSuit) TestInstanceEvent(c *check.C) {
// 	defer util.DeleteDir(util.BackupDir)
// 	log.Printf("Start to TestAddInstanceEvent")

// 	cfg := config.NewDefaultConfiguration([]string{"127.0.0.1:8008"})
// 	cfg.GetConsumer().GetLocalCache().SetServiceExpireTime(time.Second * 5)
// 	cfg.GetConsumer().GetLocalCache().SetServiceRefreshInterval(time.Second * 1)
// 	cfg.GetConsumer().GetLocalCache().SetStartUseFileCache(false)
// 	consumer, err := api.NewConsumerAPIByConfig(cfg)
// 	c.Assert(err, check.IsNil)
// 	defer consumer.Destroy()

// 	key := model.ServiceKey{
// 		Namespace: consumerNamespace,
// 		Service:   consumerService,
// 	}
// 	watchReq := api.WatchServiceRequest{}
// 	watchReq.Key = key
// 	watchRsp, err := consumer.WatchService(&watchReq)
// 	c.Assert(err, check.IsNil)
// 	channel := watchRsp.EventChannel
// 	c.Assert(channel, check.NotNil)
// 	time.Sleep(time.Second * 3)

// 	addIns := t.addInstance()[0]
// 	_ = addIns
// 	time.Sleep(time.Second * 3)
// 	event := <-channel
// 	eventStr, _ := json.Marshal(event)
// 	c.Logf("receive instance event : %s", string(eventStr))
// 	// event, err := t.GetInstanceEvent(channel)
// 	// c.Assert(event, check.NotNil)
// 	c.Assert(event.GetSubScribeEventType(), check.Equals, api.EventInstance)
// 	insEvent := event.(*model.InstanceEvent)
// 	c.Assert(insEvent.AddEvent, check.NotNil)
// 	c.Assert(insEvent.AddEvent.Instances[0].GetId(), check.Equals, addIns.GetId().Value)

// 	request := &api.GetOneInstanceRequest{}
// 	request.FlowID = 1111
// 	request.Namespace = consumerNamespace
// 	request.Service = consumerService
// 	c.Assert(err, check.IsNil)
// 	resp, err := consumer.GetOneInstance(request)
// 	id := resp.GetInstances()[0].GetId()

// 	newWeight := resp.GetInstances()[0].GetWeight() - 1
// 	t.mockServer.UpdateServerInstanceWeight(consumerNamespace, consumerService, id, uint32(newWeight))
// 	time.Sleep(time.Second * 5)
// 	event, err = t.GetInstanceEvent(channel)
// 	c.Assert(event, check.NotNil)
// 	c.Assert(event.GetSubScribeEventType(), check.Equals, api.EventInstance)
// 	insEvent = event.(*model.InstanceEvent)
// 	c.Assert(insEvent.UpdateEvent, check.NotNil)
// 	c.Assert(insEvent.UpdateEvent.UpdateList[0].After.GetId(), check.Equals, id)
// 	c.Assert(insEvent.UpdateEvent.UpdateList[0].After.GetWeight(), check.Equals, newWeight)
// 	c.Assert(insEvent.UpdateEvent.UpdateList[0].Before.GetWeight(), check.Equals, resp.GetInstances()[0].GetWeight())

// 	t.mockServer.DeleteServerInstance(consumerNamespace, consumerService, id)
// 	time.Sleep(time.Second * 5)
// 	event, err = t.GetInstanceEvent(channel)
// 	c.Assert(event, check.NotNil)
// 	c.Assert(event.GetSubScribeEventType(), check.Equals, api.EventInstance)
// 	insEvent = event.(*model.InstanceEvent)
// 	c.Assert(insEvent.DeleteEvent, check.NotNil)
// 	c.Assert(insEvent.DeleteEvent.Instances[0].GetId(), check.Equals, id)
// }

func registerRouteRuleByFile(mockServer mock.NamingServer, svc *service_manage.Service, path string) error {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	route := &traffic_manage.Routing{}
	if err = jsonpb.UnmarshalString(string(buf), route); err != nil {
		return err
	}
	return mockServer.RegisterRouteRule(svc, route)
}

// TestWatchExpired 测试订阅过期
func (t *EventSubscribeSuit) TestWatchExpired(c *check.C) {
	fmt.Println("-----------------TestWatchExpired")
	defer util.DeleteDir(util.BackupDir)
	serviceName := "InboundAddAndDelete"
	namespace := "Production"
	Instances := make([]*service_manage.Instance, 0, 2)

	Instances = append(Instances, &service_manage.Instance{
		Id:        &wrappers.StringValue{Value: uuid.New().String()},
		Service:   &wrappers.StringValue{Value: serviceName},
		Namespace: &wrappers.StringValue{Value: namespace},
		Host:      &wrappers.StringValue{Value: "127.0.0.1"},
		Port:      &wrappers.UInt32Value{Value: uint32(10030)},
		Weight:    &wrappers.UInt32Value{Value: 100},
		Metadata: map[string]string{
			"env": "formal1",
		},
	})
	Instances = append(Instances, &service_manage.Instance{
		Id:        &wrappers.StringValue{Value: uuid.New().String()},
		Service:   &wrappers.StringValue{Value: serviceName},
		Namespace: &wrappers.StringValue{Value: namespace},
		Host:      &wrappers.StringValue{Value: "127.0.0.1"},
		Port:      &wrappers.UInt32Value{Value: uint32(10031)},
		Weight:    &wrappers.UInt32Value{Value: 100},
		Metadata: map[string]string{
			"env": "formal2",
		},
	})

	service := &service_manage.Service{
		Name:      &wrappers.StringValue{Value: serviceName},
		Namespace: &wrappers.StringValue{Value: namespace},
		Token:     &wrappers.StringValue{Value: uuid.New().String()},
	}
	t.mockServer.RegisterService(service)
	if len(Instances) > 0 {
		t.mockServer.RegisterServiceInstances(service, Instances)
	}
	err := registerRouteRuleByFile(t.mockServer, service, "testdata/route_rule/inbound_add_delete.json")
	c.Assert(err, check.IsNil)

	cfg := config.NewDefaultConfiguration([]string{"127.0.0.1:8008"})
	cfg.GetConsumer().GetLocalCache().SetServiceExpireTime(time.Second * 5)
	cfg.GetConsumer().GetLocalCache().SetStartUseFileCache(false)
	consumer, err := api.NewConsumerAPIByConfig(cfg)
	defer consumer.Destroy()
	c.Assert(err, check.IsNil)

	watchReq := api.WatchServiceRequest{}
	watchReq.Key = model.ServiceKey{
		Namespace: namespace,
		Service:   serviceName,
	}
	watchRsp, err := consumer.WatchService(&watchReq)
	c.Assert(err, check.IsNil)
	_ = watchRsp

	request := &api.GetOneInstanceRequest{}
	request.FlowID = 1111
	request.Namespace = namespace
	request.Service = serviceName
	request.SourceService = &model.ServiceInfo{
		Service:   serviceName,
		Namespace: namespace,
		Metadata: map[string]string{
			"env": "formal1",
		},
	}
	c.Assert(err, check.IsNil)
	resp, err := consumer.GetOneInstance(request)
	c.Assert(err, check.IsNil)
	c.Assert(len(resp.Instances), check.Equals, 1)
	c.Assert(resp.Instances[0].GetMetadata()["env"], check.Equals, "formal1")

	time.Sleep(time.Second * 10)
	for i := 0; i < 100; i++ {
		resp, err = consumer.GetOneInstance(request)
		c.Assert(err, check.IsNil)
		c.Assert(len(resp.Instances), check.Equals, 1)
		c.Assert(resp.Instances[0].GetMetadata()["env"], check.Equals, "formal1")
	}
}
