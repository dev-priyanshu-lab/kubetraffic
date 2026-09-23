# syntax=docker/dockerfile:1
#
# Build context is the REPO ROOT (not control-plane/), because the protobuf
# contract at proto/kubetraffic/v1/route.proto is shared with the Go
# controller and lives outside this module:
#   docker build -f control-plane/Dockerfile -t kubetraffic/control-plane:dev .

FROM maven:3.9-eclipse-temurin-21 AS build
WORKDIR /src
COPY control-plane/pom.xml ./pom.xml
RUN mvn -B -q -Dmaven.test.skip=true dependency:resolve-plugins dependency:resolve || true
COPY control-plane/src ./src
COPY proto /proto
RUN mvn -B -q -DskipTests -Dproto.source.dir=/proto package

FROM gcr.io/distroless/java21-debian12:nonroot
COPY --from=build /src/target/control-plane.jar /app.jar
EXPOSE 8080 8081 9090
USER 65532:65532
ENTRYPOINT ["java", "-XX:MaxRAMPercentage=75", "-jar", "/app.jar"]
