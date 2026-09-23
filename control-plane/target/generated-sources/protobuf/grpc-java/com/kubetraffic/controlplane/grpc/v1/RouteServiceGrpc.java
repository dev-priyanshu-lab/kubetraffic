package com.kubetraffic.controlplane.grpc.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@javax.annotation.Generated(
    value = "by gRPC proto compiler (version 1.68.1)",
    comments = "Source: kubetraffic/v1/route.proto")
@io.grpc.stub.annotations.GrpcGenerated
public final class RouteServiceGrpc {

  private RouteServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "kubetraffic.v1.RouteService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.kubetraffic.controlplane.grpc.v1.RouteSpec,
      com.kubetraffic.controlplane.grpc.v1.RouteConfig> getRegisterRouteMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "RegisterRoute",
      requestType = com.kubetraffic.controlplane.grpc.v1.RouteSpec.class,
      responseType = com.kubetraffic.controlplane.grpc.v1.RouteConfig.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.kubetraffic.controlplane.grpc.v1.RouteSpec,
      com.kubetraffic.controlplane.grpc.v1.RouteConfig> getRegisterRouteMethod() {
    io.grpc.MethodDescriptor<com.kubetraffic.controlplane.grpc.v1.RouteSpec, com.kubetraffic.controlplane.grpc.v1.RouteConfig> getRegisterRouteMethod;
    if ((getRegisterRouteMethod = RouteServiceGrpc.getRegisterRouteMethod) == null) {
      synchronized (RouteServiceGrpc.class) {
        if ((getRegisterRouteMethod = RouteServiceGrpc.getRegisterRouteMethod) == null) {
          RouteServiceGrpc.getRegisterRouteMethod = getRegisterRouteMethod =
              io.grpc.MethodDescriptor.<com.kubetraffic.controlplane.grpc.v1.RouteSpec, com.kubetraffic.controlplane.grpc.v1.RouteConfig>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "RegisterRoute"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.kubetraffic.controlplane.grpc.v1.RouteSpec.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.kubetraffic.controlplane.grpc.v1.RouteConfig.getDefaultInstance()))
              .setSchemaDescriptor(new RouteServiceMethodDescriptorSupplier("RegisterRoute"))
              .build();
        }
      }
    }
    return getRegisterRouteMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.kubetraffic.controlplane.grpc.v1.RouteRef,
      com.google.protobuf.Empty> getDeleteRouteMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "DeleteRoute",
      requestType = com.kubetraffic.controlplane.grpc.v1.RouteRef.class,
      responseType = com.google.protobuf.Empty.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.kubetraffic.controlplane.grpc.v1.RouteRef,
      com.google.protobuf.Empty> getDeleteRouteMethod() {
    io.grpc.MethodDescriptor<com.kubetraffic.controlplane.grpc.v1.RouteRef, com.google.protobuf.Empty> getDeleteRouteMethod;
    if ((getDeleteRouteMethod = RouteServiceGrpc.getDeleteRouteMethod) == null) {
      synchronized (RouteServiceGrpc.class) {
        if ((getDeleteRouteMethod = RouteServiceGrpc.getDeleteRouteMethod) == null) {
          RouteServiceGrpc.getDeleteRouteMethod = getDeleteRouteMethod =
              io.grpc.MethodDescriptor.<com.kubetraffic.controlplane.grpc.v1.RouteRef, com.google.protobuf.Empty>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "DeleteRoute"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.kubetraffic.controlplane.grpc.v1.RouteRef.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.google.protobuf.Empty.getDefaultInstance()))
              .setSchemaDescriptor(new RouteServiceMethodDescriptorSupplier("DeleteRoute"))
              .build();
        }
      }
    }
    return getDeleteRouteMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.kubetraffic.controlplane.grpc.v1.RouteRef,
      com.kubetraffic.controlplane.grpc.v1.RouteConfig> getGetRouteConfigMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetRouteConfig",
      requestType = com.kubetraffic.controlplane.grpc.v1.RouteRef.class,
      responseType = com.kubetraffic.controlplane.grpc.v1.RouteConfig.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.kubetraffic.controlplane.grpc.v1.RouteRef,
      com.kubetraffic.controlplane.grpc.v1.RouteConfig> getGetRouteConfigMethod() {
    io.grpc.MethodDescriptor<com.kubetraffic.controlplane.grpc.v1.RouteRef, com.kubetraffic.controlplane.grpc.v1.RouteConfig> getGetRouteConfigMethod;
    if ((getGetRouteConfigMethod = RouteServiceGrpc.getGetRouteConfigMethod) == null) {
      synchronized (RouteServiceGrpc.class) {
        if ((getGetRouteConfigMethod = RouteServiceGrpc.getGetRouteConfigMethod) == null) {
          RouteServiceGrpc.getGetRouteConfigMethod = getGetRouteConfigMethod =
              io.grpc.MethodDescriptor.<com.kubetraffic.controlplane.grpc.v1.RouteRef, com.kubetraffic.controlplane.grpc.v1.RouteConfig>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetRouteConfig"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.kubetraffic.controlplane.grpc.v1.RouteRef.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.kubetraffic.controlplane.grpc.v1.RouteConfig.getDefaultInstance()))
              .setSchemaDescriptor(new RouteServiceMethodDescriptorSupplier("GetRouteConfig"))
              .build();
        }
      }
    }
    return getGetRouteConfigMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static RouteServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<RouteServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<RouteServiceStub>() {
        @java.lang.Override
        public RouteServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new RouteServiceStub(channel, callOptions);
        }
      };
    return RouteServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static RouteServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<RouteServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<RouteServiceBlockingStub>() {
        @java.lang.Override
        public RouteServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new RouteServiceBlockingStub(channel, callOptions);
        }
      };
    return RouteServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static RouteServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<RouteServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<RouteServiceFutureStub>() {
        @java.lang.Override
        public RouteServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new RouteServiceFutureStub(channel, callOptions);
        }
      };
    return RouteServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     * <pre>
     * RegisterRoute idempotently upserts a route's desired spec and returns the
     * current effective configuration.
     * </pre>
     */
    default void registerRoute(com.kubetraffic.controlplane.grpc.v1.RouteSpec request,
        io.grpc.stub.StreamObserver<com.kubetraffic.controlplane.grpc.v1.RouteConfig> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRegisterRouteMethod(), responseObserver);
    }

    /**
     * <pre>
     * DeleteRoute removes a route the controller no longer manages.
     * </pre>
     */
    default void deleteRoute(com.kubetraffic.controlplane.grpc.v1.RouteRef request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeleteRouteMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetRouteConfig polls the effective configuration. Used as a fallback when
     * the decision stream is unavailable.
     * </pre>
     */
    default void getRouteConfig(com.kubetraffic.controlplane.grpc.v1.RouteRef request,
        io.grpc.stub.StreamObserver<com.kubetraffic.controlplane.grpc.v1.RouteConfig> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetRouteConfigMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service RouteService.
   */
  public static abstract class RouteServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return RouteServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service RouteService.
   */
  public static final class RouteServiceStub
      extends io.grpc.stub.AbstractAsyncStub<RouteServiceStub> {
    private RouteServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected RouteServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new RouteServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * RegisterRoute idempotently upserts a route's desired spec and returns the
     * current effective configuration.
     * </pre>
     */
    public void registerRoute(com.kubetraffic.controlplane.grpc.v1.RouteSpec request,
        io.grpc.stub.StreamObserver<com.kubetraffic.controlplane.grpc.v1.RouteConfig> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRegisterRouteMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * DeleteRoute removes a route the controller no longer manages.
     * </pre>
     */
    public void deleteRoute(com.kubetraffic.controlplane.grpc.v1.RouteRef request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeleteRouteMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetRouteConfig polls the effective configuration. Used as a fallback when
     * the decision stream is unavailable.
     * </pre>
     */
    public void getRouteConfig(com.kubetraffic.controlplane.grpc.v1.RouteRef request,
        io.grpc.stub.StreamObserver<com.kubetraffic.controlplane.grpc.v1.RouteConfig> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetRouteConfigMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service RouteService.
   */
  public static final class RouteServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<RouteServiceBlockingStub> {
    private RouteServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected RouteServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new RouteServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * RegisterRoute idempotently upserts a route's desired spec and returns the
     * current effective configuration.
     * </pre>
     */
    public com.kubetraffic.controlplane.grpc.v1.RouteConfig registerRoute(com.kubetraffic.controlplane.grpc.v1.RouteSpec request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRegisterRouteMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteRoute removes a route the controller no longer manages.
     * </pre>
     */
    public com.google.protobuf.Empty deleteRoute(com.kubetraffic.controlplane.grpc.v1.RouteRef request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeleteRouteMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetRouteConfig polls the effective configuration. Used as a fallback when
     * the decision stream is unavailable.
     * </pre>
     */
    public com.kubetraffic.controlplane.grpc.v1.RouteConfig getRouteConfig(com.kubetraffic.controlplane.grpc.v1.RouteRef request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetRouteConfigMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service RouteService.
   */
  public static final class RouteServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<RouteServiceFutureStub> {
    private RouteServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected RouteServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new RouteServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * RegisterRoute idempotently upserts a route's desired spec and returns the
     * current effective configuration.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.kubetraffic.controlplane.grpc.v1.RouteConfig> registerRoute(
        com.kubetraffic.controlplane.grpc.v1.RouteSpec request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRegisterRouteMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * DeleteRoute removes a route the controller no longer manages.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.google.protobuf.Empty> deleteRoute(
        com.kubetraffic.controlplane.grpc.v1.RouteRef request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeleteRouteMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetRouteConfig polls the effective configuration. Used as a fallback when
     * the decision stream is unavailable.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.kubetraffic.controlplane.grpc.v1.RouteConfig> getRouteConfig(
        com.kubetraffic.controlplane.grpc.v1.RouteRef request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetRouteConfigMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_REGISTER_ROUTE = 0;
  private static final int METHODID_DELETE_ROUTE = 1;
  private static final int METHODID_GET_ROUTE_CONFIG = 2;

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
        case METHODID_REGISTER_ROUTE:
          serviceImpl.registerRoute((com.kubetraffic.controlplane.grpc.v1.RouteSpec) request,
              (io.grpc.stub.StreamObserver<com.kubetraffic.controlplane.grpc.v1.RouteConfig>) responseObserver);
          break;
        case METHODID_DELETE_ROUTE:
          serviceImpl.deleteRoute((com.kubetraffic.controlplane.grpc.v1.RouteRef) request,
              (io.grpc.stub.StreamObserver<com.google.protobuf.Empty>) responseObserver);
          break;
        case METHODID_GET_ROUTE_CONFIG:
          serviceImpl.getRouteConfig((com.kubetraffic.controlplane.grpc.v1.RouteRef) request,
              (io.grpc.stub.StreamObserver<com.kubetraffic.controlplane.grpc.v1.RouteConfig>) responseObserver);
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
          getRegisterRouteMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.kubetraffic.controlplane.grpc.v1.RouteSpec,
              com.kubetraffic.controlplane.grpc.v1.RouteConfig>(
                service, METHODID_REGISTER_ROUTE)))
        .addMethod(
          getDeleteRouteMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.kubetraffic.controlplane.grpc.v1.RouteRef,
              com.google.protobuf.Empty>(
                service, METHODID_DELETE_ROUTE)))
        .addMethod(
          getGetRouteConfigMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.kubetraffic.controlplane.grpc.v1.RouteRef,
              com.kubetraffic.controlplane.grpc.v1.RouteConfig>(
                service, METHODID_GET_ROUTE_CONFIG)))
        .build();
  }

  private static abstract class RouteServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    RouteServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.kubetraffic.controlplane.grpc.v1.RouteProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("RouteService");
    }
  }

  private static final class RouteServiceFileDescriptorSupplier
      extends RouteServiceBaseDescriptorSupplier {
    RouteServiceFileDescriptorSupplier() {}
  }

  private static final class RouteServiceMethodDescriptorSupplier
      extends RouteServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    RouteServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (RouteServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new RouteServiceFileDescriptorSupplier())
              .addMethod(getRegisterRouteMethod())
              .addMethod(getDeleteRouteMethod())
              .addMethod(getGetRouteConfigMethod())
              .build();
        }
      }
    }
    return result;
  }
}
