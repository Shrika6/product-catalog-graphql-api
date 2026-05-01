import http from 'k6/http';
import { check, fail, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const GRAPHQL_URL = `${BASE_URL}/query`;
const PRODUCT_ID = __ENV.PRODUCT_ID || '';
const TARGET_RPS = Number(__ENV.TARGET_RPS || 100);
const DURATION = __ENV.DURATION || '2m';
const PRE_VUS = Number(__ENV.PRE_VUS || 60);
const MAX_VUS = Number(__ENV.MAX_VUS || 600);

if (!PRODUCT_ID) {
  fail('PRODUCT_ID is required');
}

const productsQuery = `
query Products {
  products(pagination:{limit:20, offset:0}, sorting:{field:PRICE, order:ASC}) {
    id
    name
    price
    category { id name }
  }
}`;

const productQuery = `
query Product($id: ID!) {
  product(id:$id) {
    id
    name
    price
    category { id name }
  }
}`;

export const options = {
  scenarios: {
    capacity_step: {
      executor: 'constant-arrival-rate',
      rate: TARGET_RPS,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: PRE_VUS,
      maxVUs: MAX_VUS,
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<1500'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
};

function gql(query, variables) {
  const res = http.post(
    GRAPHQL_URL,
    JSON.stringify({ query, variables }),
    { headers: { 'Content-Type': 'application/json' } }
  );

  check(res, {
    'status 200': (r) => r.status === 200,
    'no graphql errors': (r) => {
      try {
        const b = JSON.parse(r.body);
        return !b.errors;
      } catch (_) {
        return false;
      }
    },
  });

  return res;
}

export default function () {
  const mix = Math.random();
  if (mix < 0.75) {
    gql(productsQuery, null);
  } else {
    gql(productQuery, { id: PRODUCT_ID });
  }
  sleep(0.05);
}
