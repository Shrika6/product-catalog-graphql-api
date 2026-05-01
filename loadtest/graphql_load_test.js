import http from 'k6/http';
import { check, fail, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const GRAPHQL_URL = `${BASE_URL}/query`;
const MODE = (__ENV.MODE || 'read-heavy').toLowerCase();
const PRODUCT_ID = __ENV.PRODUCT_ID || '';
const CATEGORY_ID = __ENV.CATEGORY_ID || '';
const JWT_TOKEN = __ENV.JWT_TOKEN || '';

if ((MODE === 'read-heavy' || MODE === 'ramp') && !PRODUCT_ID) {
  fail('PRODUCT_ID is required for read-heavy/ramp mode');
}
if (MODE === 'mixed' && (!PRODUCT_ID || !CATEGORY_ID || !JWT_TOKEN)) {
  fail('PRODUCT_ID, CATEGORY_ID, and JWT_TOKEN are required for mixed mode');
}

const headers = {
  'Content-Type': 'application/json',
};

function gqlRequest(query, variables, authToken = '') {
  const requestHeaders = { ...headers };
  if (authToken) {
    requestHeaders.Authorization = `Bearer ${authToken}`;
  }

  const res = http.post(
    GRAPHQL_URL,
    JSON.stringify({ query, variables }),
    {
      headers: requestHeaders,
      tags: {
        mode: MODE,
      },
    }
  );

  const ok = check(res, {
    'status is 200': (r) => r.status === 200,
    'graphql has no errors': (r) => {
      try {
        const body = JSON.parse(r.body);
        return !body.errors;
      } catch (e) {
        return false;
      }
    },
  });

  if (!ok) {
    console.error(`Request failed (${res.status}): ${res.body}`);
  }

  return res;
}

const productsQuery = `
query Products($filter: ProductFilterInput, $pagination: PaginationInput, $sorting: ProductSortInput) {
  products(filter: $filter, pagination: $pagination, sorting: $sorting) {
    id
    name
    price
    currency
    stockQuantity
    category { id name }
  }
}`;

const productQuery = `
query Product($id: ID!) {
  product(id: $id) {
    id
    name
    description
    price
    currency
    stockQuantity
    category { id name }
  }
}`;

const createProductMutation = `
mutation CreateProduct($input: CreateProductInput!) {
  createProduct(input: $input) { id name }
}`;

const updateProductMutation = `
mutation UpdateProduct($id: ID!, $input: UpdateProductInput!) {
  updateProduct(id: $id, input: $input) { id name price stockQuantity }
}`;

export const options = {
  scenarios: {
    read_heavy: {
      executor: 'constant-arrival-rate',
      exec: 'readHeavyScenario',
      rate: 80,
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 40,
      maxVUs: 200,
      tags: { scenario: 'read-heavy' },
      startTime: MODE === 'read-heavy' ? '0s' : '1000h',
    },
    mixed: {
      executor: 'constant-arrival-rate',
      exec: 'mixedScenario',
      rate: 40,
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 30,
      maxVUs: 150,
      tags: { scenario: 'mixed' },
      startTime: MODE === 'mixed' ? '0s' : '1000h',
    },
    ramp_until_saturation: {
      executor: 'ramping-arrival-rate',
      exec: 'rampScenario',
      startRate: 20,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 400,
      stages: [
        { target: 50, duration: '2m' },
        { target: 100, duration: '2m' },
        { target: 150, duration: '2m' },
        { target: 200, duration: '2m' },
        { target: 250, duration: '2m' },
      ],
      tags: { scenario: 'ramp' },
      startTime: MODE === 'ramp' ? '0s' : '1000h',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.02'],
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    checks: ['rate>0.98'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
};

export function readHeavyScenario() {
  const readType = Math.random();
  if (readType < 0.7) {
    gqlRequest(productsQuery, {
      filter: { nameSearch: 'key' },
      pagination: { limit: 20, offset: 0 },
      sorting: { field: 'PRICE', order: 'ASC' },
    });
  } else {
    gqlRequest(productQuery, { id: PRODUCT_ID });
  }
  sleep(0.1);
}

export function mixedScenario() {
  const choice = Math.random();

  if (choice < 0.75) {
    readHeavyScenario();
    return;
  }

  if (choice < 0.9) {
    gqlRequest(
      createProductMutation,
      {
        input: {
          name: `k6-product-${__VU}-${Date.now()}`,
          description: 'Created during mixed load scenario',
          price: 89.99,
          currency: 'USD',
          categoryId: CATEGORY_ID,
          stockQuantity: 50,
        },
      },
      JWT_TOKEN
    );
    return;
  }

  gqlRequest(
    updateProductMutation,
    {
      id: PRODUCT_ID,
      input: {
        price: 109.99,
      },
    },
    JWT_TOKEN
  );
}

export function rampScenario() {
  readHeavyScenario();
}
