/**
 * ViewAura GraphQL BFF — Server entrypoint
 *
 * Stack:
 *   Apollo Server 4  (expressMiddleware) — HTTP queries + mutations
 *   graphql-ws / ws  — WebSocket subscriptions (separate port via upgrade)
 *   Express 4        — HTTP framework
 *
 * Subscriptions use a separate WebSocket upgrade on the same HTTP server.
 * The graphql-ws library handles the graphql-transport-ws subprotocol
 * which Apollo Client uses natively from v3+.
 *
 * Port: 7001 (from env PORT or default)
 *
 * Health endpoint: GET /health → 200 { status: "ok" }
 * GraphQL endpoint: POST /graphql
 * Subscriptions:   ws://host:7001/graphql
 */

import "dotenv/config";
import http from "node:http";
import { ApolloServer } from "@apollo/server";
import { expressMiddleware } from "@apollo/server/express4";
import { ApolloServerPluginDrainHttpServer } from "@apollo/server/plugin/drainHttpServer";
import { makeExecutableSchema } from "@graphql-tools/schema";
import { WebSocketServer } from "ws";
import { useServer } from "graphql-ws/lib/use/ws";
import express from "express";
import cors from "cors";
import bodyParser from "body-parser";
import { typeDefs }  from "./schema/typeDefs.js";
import { resolvers } from "./resolvers/index.js";
import { buildContext, type ViewAuraContext } from "./context/index.js";
import { logger } from "./logger.js";

const PORT = parseInt(process.env.PORT ?? "7001", 10);

async function main() {
  const app        = express();
  const httpServer = http.createServer(app);

  // ── Build executable schema ────────────────────────────────────────────────
  const schema = makeExecutableSchema({ typeDefs, resolvers });

  // ── WebSocket server (subscriptions) ──────────────────────────────────────
  const wsServer = new WebSocketServer({
    server: httpServer,
    path:   "/graphql",
  });

  // graphql-ws handles the WebSocket lifecycle. Context is built the same
  // way as for HTTP — token extracted from the connectionParams.
  const serverCleanup = useServer(
    {
      schema,
      context: (ctx) => {
        // WS clients send auth via connectionParams: { authorization: "Bearer …" }
        const authHeader = (ctx.connectionParams?.authorization ?? "") as string;
        const fakeReq = {
          headers: {
            authorization: authHeader,
            "x-ab-experiments": "",
            "x-request-id": crypto.randomUUID(),
          },
        } as unknown as import("node:http").IncomingMessage;
        return buildContext(fakeReq);
      },
    },
    wsServer,
  );

  // ── Apollo Server ─────────────────────────────────────────────────────────
  const server = new ApolloServer<ViewAuraContext>({
    schema,
    plugins: [
      // Graceful shutdown — drain HTTP connections before shutting down
      ApolloServerPluginDrainHttpServer({ httpServer }),
      // Graceful shutdown — drain WS connections too
      {
        async serverWillStart() {
          return {
            async drainServer() {
              await serverCleanup.dispose();
            },
          };
        },
      },
    ],
    formatError: (formattedError, error) => {
      // Log server-side errors but don't expose internal details to clients
      logger.error({ err: error }, "GraphQL error");
      if (formattedError.extensions?.code === "INTERNAL_SERVER_ERROR") {
        return { ...formattedError, message: "Internal server error" };
      }
      return formattedError;
    },
    introspection: process.env.NODE_ENV !== "production",
  });

  await server.start();

  // ── Express middleware ─────────────────────────────────────────────────────
  app.use(
    cors<cors.CorsRequest>({
      origin: [
        "https://viewaura.com",
        "https://www.viewaura.com",
        "https://studio.viewaura.com",
        "http://localhost:3000",  // local web dev
      ],
      credentials: true,
    }),
  );

  app.get("/health", (_req, res) => {
    res.json({ status: "ok", service: "graphql-bff", port: PORT });
  });

  app.use(
    "/graphql",
    bodyParser.json({ limit: "1mb" }),
    expressMiddleware(server, {
      context: async ({ req }) => buildContext(req),
    }),
  );

  // ── Start ─────────────────────────────────────────────────────────────────
  await new Promise<void>((resolve) => httpServer.listen({ port: PORT }, resolve));

  logger.info(
    { port: PORT, env: process.env.NODE_ENV },
    "graphql-bff listening",
  );
  logger.info(`GraphQL: http://localhost:${PORT}/graphql`);
  logger.info(`Subscriptions: ws://localhost:${PORT}/graphql`);
}

main().catch((err) => {
  logger.fatal({ err }, "failed to start graphql-bff");
  process.exit(1);
});