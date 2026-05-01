(function () {
  const form = document.getElementById("search-form");
  const searchInput = document.getElementById("nameSearch");
  const minPriceInput = document.getElementById("minPrice");
  const maxPriceInput = document.getElementById("maxPrice");
  const sortFieldSelect = document.getElementById("sortField");
  const sortOrderSelect = document.getElementById("sortOrder");
  const searchButton = document.getElementById("searchButton");
  const statusEl = document.getElementById("status");
  const validationMessageEl = document.getElementById("validation-message");
  const resultsEl = document.getElementById("results");
  const emptyStateEl = document.getElementById("empty-state");
  const resultsHeading = document.getElementById("results-heading");

  function setStatus(message) {
    statusEl.textContent = message;
  }

  function setValidationMessage(message) {
    validationMessageEl.textContent = message || "";
  }

  function setInputValidity(input, valid) {
    input.setAttribute("aria-invalid", valid ? "false" : "true");
  }

  function validatePriceRange(minPrice, maxPrice) {
    setValidationMessage("");
    setInputValidity(minPriceInput, true);
    setInputValidity(maxPriceInput, true);

    if (minPrice && Number(minPrice) < 0) {
      setValidationMessage("Minimum price cannot be negative.");
      setInputValidity(minPriceInput, false);
      minPriceInput.focus();
      return false;
    }

    if (maxPrice && Number(maxPrice) < 0) {
      setValidationMessage("Maximum price cannot be negative.");
      setInputValidity(maxPriceInput, false);
      maxPriceInput.focus();
      return false;
    }

    if (minPrice && maxPrice && Number(minPrice) > Number(maxPrice)) {
      setValidationMessage("Minimum price cannot be greater than maximum price.");
      setInputValidity(minPriceInput, false);
      setInputValidity(maxPriceInput, false);
      minPriceInput.focus();
      return false;
    }

    return true;
  }

  function filterSummary(search, minPrice, maxPrice) {
    const chunks = [];
    if (search) chunks.push("name/description contains '" + search + "'");
    if (minPrice) chunks.push("min price " + minPrice);
    if (maxPrice) chunks.push("max price " + maxPrice);
    return chunks.length ? chunks.join(", ") : "no filters";
  }

  function renderProducts(products) {
    resultsEl.innerHTML = "";

    if (!products.length) {
      emptyStateEl.style.display = "block";
      return;
    }

    emptyStateEl.style.display = "none";

    products.forEach(function (product) {
      const card = document.createElement("article");
      card.className = "card";
      card.setAttribute("role", "listitem");

      const title = document.createElement("h3");
      title.textContent = product.name;

      const description = document.createElement("p");
      description.className = "meta";
      description.textContent = product.description || "No description provided.";

      const pricing = document.createElement("p");
      pricing.className = "meta";
      pricing.textContent = product.currency + " " + Number(product.price).toFixed(2) + " | Stock: " + product.stockQuantity;

      const category = document.createElement("p");
      category.className = "meta";
      category.textContent = "Category: " + (product.category ? product.category.name : "Uncategorized");

      card.appendChild(title);
      card.appendChild(description);
      card.appendChild(pricing);
      card.appendChild(category);
      resultsEl.appendChild(card);
    });
  }

  async function fetchProducts(event) {
    event.preventDefault();

    const search = searchInput.value.trim();
    const minPrice = minPriceInput.value.trim();
    const maxPrice = maxPriceInput.value.trim();
    const sortField = sortFieldSelect.value;
    const sortOrder = sortOrderSelect.value;

    if (!validatePriceRange(minPrice, maxPrice)) {
      return;
    }

    searchButton.disabled = true;
    searchButton.textContent = "Loading...";
    searchButton.setAttribute("aria-busy", "true");
    resultsEl.setAttribute("aria-busy", "true");
    setStatus("Loading products...");

    const filter = {};
    if (search) filter.nameSearch = search;
    if (minPrice) filter.minPrice = Number(minPrice);
    if (maxPrice) filter.maxPrice = Number(maxPrice);

    const payload = {
      query: "query Products($filter: ProductFilterInput, $pagination: PaginationInput, $sorting: ProductSortInput) { products(filter: $filter, pagination: $pagination, sorting: $sorting) { id name description price currency stockQuantity category { id name } } }",
      variables: {
        filter: Object.keys(filter).length ? filter : null,
        pagination: { limit: 20, offset: 0 },
        sorting: { field: sortField, order: sortOrder }
      }
    };

    try {
      const response = await fetch("/query", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
      const json = await response.json();

      if (json.errors && json.errors.length) {
        setStatus("Error: " + json.errors[0].message);
        renderProducts([]);
        return;
      }

      const products = (json.data && json.data.products) || [];
      renderProducts(products);
      setStatus("Loaded " + products.length + " products with " + filterSummary(search, minPrice, maxPrice) + ".");

      window.requestAnimationFrame(function () {
        resultsHeading.focus();
      });
    } catch (error) {
      setStatus("Network error while loading products.");
      renderProducts([]);
    } finally {
      searchButton.disabled = false;
      searchButton.textContent = "Search";
      searchButton.setAttribute("aria-busy", "false");
      resultsEl.setAttribute("aria-busy", "false");
    }
  }

  setInputValidity(minPriceInput, true);
  setInputValidity(maxPriceInput, true);
  resultsEl.setAttribute("aria-busy", "false");

  form.addEventListener("submit", fetchProducts);
})();
