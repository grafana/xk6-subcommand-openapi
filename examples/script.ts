// Load test of QuickPizza with the client that `k6 x openapi` generates from its OpenAPI descriptor.
// See README.md in this directory for how to generate the client and run this script.
import { check, sleep } from "k6";
import { QuickPizzaAPIClient } from "./quickpizza-client.ts";

export const options = {
  vus: 2,
  duration: "10s",
  thresholds: {
    checks: ["rate>0.99"],
    http_req_failed: ["rate<0.01"],
  },
};

const client = new QuickPizzaAPIClient({
  baseUrl: __ENV.BASE_URL || "https://quickpizza.grafana.com",
  // QuickPizza accepts any token of 16 characters.
  commonK6Params: { headers: { Authorization: "token abcdef0123456789" } },
});

export default async function (): Promise<void> {
  const recommended = await client.getPizzaRecommendation({
    body: { maxCaloriesPerSlice: 800, mustBeVegetarian: true, minNumberOfToppings: 2 },
    k6Params: { tags: { name: "getPizzaRecommendation" } },
  });
  check(recommended, { "pizza recommended": (res) => res.status === 200 });
  if (recommended.status !== 200) {
    return;
  }

  // Checking the status narrowed recommended.data to a PizzaRecommendation.
  const id = recommended.data.pizza?.id;
  check(recommended.data, { "pizza is vegetarian": (data) => data.vegetarian === true });
  if (id === undefined) {
    return;
  }

  const pizza = await client.getPizzaById(id, { k6Params: { tags: { name: "getPizzaById" } } });
  check(pizza, { "pizza found": (res) => res.status === 200 && res.data.id === id });

  sleep(1);
}
