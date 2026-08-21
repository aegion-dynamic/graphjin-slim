// GraphQL client. The endpoint comes from the "?endpoint=" query parameter
// appended by the webui.Handler root redirect, falling back to the standard
// service path.

function graphqlEndpoint(): string {
  const ep = new URLSearchParams(window.location.search).get('endpoint');
  return ep && ep.startsWith('/') ? ep : '/api/v1/graphql';
}

export interface GQLResponse<T> {
  data?: T;
  errors?: { message: string }[];
}

export async function runQuery<T = unknown>(
  query: string,
  variables?: Record<string, unknown>,
): Promise<GQLResponse<T>> {
  const res = await fetch(graphqlEndpoint(), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: JSON.stringify({ query, variables: variables ?? {} }),
  });
  if (!res.ok) {
    throw new Error(`HTTP ${res.status} from ${graphqlEndpoint()}`);
  }
  return res.json();
}

export function endpointURL(): string {
  return graphqlEndpoint();
}
