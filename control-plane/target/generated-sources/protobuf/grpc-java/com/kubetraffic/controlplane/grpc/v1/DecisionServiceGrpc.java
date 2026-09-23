package com.kubetraffic.controlplane.grpc.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@javax.annotation.Generated(
    value = "by gRPC proto compiler (version 1.68.1)",
    comments = "Source: kubetraffic/v1/route.proto")
@io.grpc.stub.annotations.GrpcGenerated
public final class DecisionServiceGrpc {

  private DecisionServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "kubetraffic.v1.DecisionService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.kubetraffic.controlplane.grpc.v1.WatchDecisionsRequest,
      com.kubetraffic.controlplane.grpc.v1.Decision> getStreamDecisionsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "StreamDecisions",
      requestType = com.kubetraffic.controlplane.grpc.v1.WatchDecisionsRequest.class,
      responseType = com.kubetraffic.controlplane.grpc.v1.Decision.class,
      methodType = io.grpc.MethodDescriptor.MethodType.SERVER_STREAMING)
  public static io.grpc.MethodDescriptor<com.kubetraffic.controlplane.grpc.v1.WatchDecisionsRequest,
      com.kubetraffic.controlplane.grpc.v1.Decision> getStreamDecisionsMethod() {
    io.grpc.MethodDescriptor<com.kubetraffic.controlplane.grpc.v1.WatchDecisionsRequest, com.kubetraffic.controlplane.grpc.v1.Decision> getStreamDecisionsMethod;
    if ((getStreamDecisionsMethod = DecisionServiceGrpc.getStreamDecisionsMethod) == null) {
      synchronized (DecisionServiceGrpc.class) {
        if ((getStreamDecisionsMethod = DecisionServiceGrpc.getStreamDecisionsMethod) == null) {
          DecisionServiceGrpc.getStreamDecisionsMethod = getStreamDecisionsMethod =
              io.grpc.MethodDescriptor.<com.kubetraffic.controlplane.grpc.v1.WatchDecisionsRequest, com.kubetraffic.controlplane.grpc.v1.Decision>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.SERVER_STREAMING)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "StreamDecisions"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.kubetraffic.controlplane.grpc.v1.WatchDecisionsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.kubetraffic.controlplane.grpc.v1.Decision.getDefaultInstance()))
              .setSchemaDescriptor(new DecisionServiceMethodDescriptorSupplier("StreamDecisions"))
              .build();
        }
      }
    }
    return getStreamDecisionsMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static DecisionServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DecisionServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DecisionServiceStub>() {
        @java.lang.Override
        public DecisionServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DecisionServiceStub(channel, callOptions);
        }
      };
    return DecisionServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static DecisionServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DecisionServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DecisionServiceBlockingStub>() {
        @java.lang.Override
        public DecisionServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DecisionServiceBlockingStub(channel, callOptions);
        }
      };
    return DecisionServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static DecisionServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DecisionServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DecisionServiceFutureStub>() {
        @java.lang.Override
        public DecisionServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DecisionServiceFutureStub(channel, callOptions);
        }
      };
    return DecisionServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     * <pre>
     * StreamDecisions pushes every effective-weight change the control plane
     * records, so controllers react without polling.
     * </pre>
     */
    default void streamDecisions(com.kubetraffic.controlplane.grpc.v1.WatchDecisionsRequest request,
        io.grpc.stub.StreamObserver<com.kubetraffic.controlplane.grpc.v1.Decision> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getStreamDecisionsMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service DecisionService.
   */
  public static abstract class DecisionServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return DecisionServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service DecisionService.
   */
  public static final class DecisionServiceStub
      extends io.grpc.stub.AbstractAsyncStub<DecisionServiceStub> {
    private DecisionServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DecisionServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DecisionServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * StreamDecisions pushes every effective-weight change the control plane
     * records, so controllers react without polling.
     * </pre>
     */
    public void streamDecisions(com.kubetraffic.controlplane.grpc.v1.WatchDecisionsRequest request,
        io.grpc.stub.StreamObserver<com.kubetraffic.controlplane.grpc.v1.Decision> responseObserver) {
      io.grpc.stub.ClientCalls.asyncServerStreamingCall(
          getChannel().newCall(getStreamDecisionsMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service DecisionService.
   */
  public static final class DecisionServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<DecisionServiceBlockingStub> {
    private DecisionServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DecisionServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DecisionServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * StreamDecisions pushes every effective-weight change the control plane
     * records, so controllers react without polling.
     * </pre>
     */
    public java.util.Iterator<com.kubetraffic.controlplane.grpc.v1.Decision> streamDecisions(
        com.kubetraffic.controlplane.grpc.v1.WatchDecisionsRequest request) {
      return io.grpc.stub.ClientCalls.blockingServerStreamingCall(
          getChannel(), getStreamDecisionsMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service DecisionService.
   */
  public static final class DecisionServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<DecisionServiceFutureStub> {
    private DecisionServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DecisionServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DecisionServiceFutureStub(channel, callOptions);
    }
  }

  private static final int METHODID_STREAM_DECISIONS = 0;

  private static final class MethodHandlers<Req, Resp> implements
      io.grpc.stub.ServerCalls.UnaryMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ServerStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ClientStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.BidiStreamingMethod<Req, Resp> {
    private final AsyncService serviceImpl;
    private final int methodId;

    MethodHandlers(AsyncService serviceImpl, int methodId) {
      this.serviceImpl = serviceImpl;
      this.methodId = methodId;
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public void invoke(Req request, io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        case METHODID_STREAM_DECISIONS:
          serviceImpl.streamDecisions((com.kubetraffic.controlplane.grpc.v1.WatchDecisionsRequest) request,
              (io.grpc.stub.StreamObserver<com.kubetraffic.controlplane.grpc.v1.Decision>) responseObserver);
          break;
        default:
          throw new AssertionError();
      }
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public io.grpc.stub.StreamObserver<Req> invoke(
        io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        default:
          throw new AssertionError();
      }
    }
  }

  public static final io.grpc.ServerServiceDefinition bindService(AsyncService service) {
    return io.grpc.ServerServiceDefinition.builder(getServiceDescriptor())
        .addMethod(
          getStreamDecisionsMethod(),
          io.grpc.stub.ServerCalls.asyncServerStreamingCall(
            new MethodHandlers<
              com.kubetraffic.controlplane.grpc.v1.WatchDecisionsRequest,
              com.kubetraffic.controlplane.grpc.v1.Decision>(
                service, METHODID_STREAM_DECISIONS)))
        .build();
  }

  private static abstract class DecisionServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    DecisionServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.kubetraffic.controlplane.grpc.v1.RouteProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("DecisionService");
    }
  }

  private static final class DecisionServiceFileDescriptorSupplier
      extends DecisionServiceBaseDescriptorSupplier {
    DecisionServiceFileDescriptorSupplier() {}
  }

  private static final class DecisionServiceMethodDescriptorSupplier
      extends DecisionServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    DecisionServiceMethodDescriptorSupplier(java.lang.String methodName) {
      this.methodName = methodName;
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.MethodDescriptor getMethodDescriptor() {
      return getServiceDescriptor().findMethodByName(methodName);
    }
  }

  private static volatile io.grpc.ServiceDescriptor serviceDescriptor;

  public static io.grpc.ServiceDescriptor getServiceDescriptor() {
    io.grpc.ServiceDescriptor result = serviceDescriptor;
    if (result == null) {
      synchronized (DecisionServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new DecisionServiceFileDescriptorSupplier())
              .addMethod(getStreamDecisionsMethod())
              .build();
        }
      }
    }
    return result;
  }
}
