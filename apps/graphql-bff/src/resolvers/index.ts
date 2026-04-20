import { GraphQLScalarType, Kind } from "graphql";
import { Query }        from "./query.js";
import { Mutation }     from "./mutation.js";
import { Subscription } from "./subscription.js";
import {
  Movie, WatchlistEntry, RecommendedMovie, FeedItem, Review,
} from "./movie.js";

const DateTimeScalar = new GraphQLScalarType({
  name:    "DateTime",
  description: "ISO-8601 datetime string",
  serialize:   (v) => String(v),
  parseValue:  (v) => String(v),
  parseLiteral: (ast) => ast.kind === Kind.STRING ? ast.value : null,
});

const JSONScalar = new GraphQLScalarType({
  name:    "JSON",
  description: "Arbitrary JSON value",
  serialize:   (v) => v,
  parseValue:  (v) => v,
  parseLiteral: (ast) => {
    if (ast.kind === Kind.STRING) {
      try { return JSON.parse(ast.value); } catch { return null; }
    }
    return null;
  },
});

export const resolvers = {
  DateTime:        DateTimeScalar,
  JSON:            JSONScalar,
  Query,
  Mutation,
  Subscription,
  Movie,
  WatchlistEntry,
  RecommendedMovie,
  FeedItem,
  Review,
};