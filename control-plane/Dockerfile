# syntax=docker/dockerfile:1

FROM maven:3.9-eclipse-temurin-21 AS build
WORKDIR /src
COPY pom.xml .
RUN mvn -B -q -Dmaven.test.skip=true dependency:resolve-plugins dependency:resolve || true
COPY src ./src
RUN mvn -B -q -DskipTests package

FROM gcr.io/distroless/java21-debian12:nonroot
COPY --from=build /src/target/control-plane.jar /app.jar
EXPOSE 8080 8081
USER 65532:65532
ENTRYPOINT ["java", "-XX:MaxRAMPercentage=75", "-jar", "/app.jar"]
