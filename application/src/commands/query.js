import { query, withGateway } from '../gateway.js';
import { connectionOptions, registerCommand, requireArg, requireInteger, requireNumber } from '../cli.js';

const GROUP = 'Query (read only)';

function evaluate(flags, functionName, args) {
  return withGateway(connectionOptions(flags), (connection) =>
    query(connection, functionName, args));
}

function registerSimpleQuery({ name, usage, summary, functionName }) {
  registerCommand({
    name,
    group: GROUP,
    usage,
    summary,
    async run({ flags }) {
      return evaluate(flags, functionName, []);
    },
  });
}

function registerLookup({ name, usage, summary, functionName, argumentName }) {
  registerCommand({
    name,
    group: GROUP,
    usage,
    summary,
    async run({ positional, flags }) {
      const value = requireArg(positional, 0, argumentName, usage);
      return evaluate(flags, functionName, [value]);
    },
  });
}


registerSimpleQuery({
  name: 'merchant-types',
  usage: 'merchant-types',
  summary: 'List the merchant type catalogue',
  functionName: 'GetAllMerchantTypes',
});

registerSimpleQuery({
  name: 'merchants',
  usage: 'merchants',
  summary: 'List every merchant',
  functionName: 'GetAllMerchants',
});

registerSimpleQuery({
  name: 'customers',
  usage: 'customers',
  summary: 'List every customer',
  functionName: 'GetAllCustomers',
});

registerSimpleQuery({
  name: 'products',
  usage: 'products',
  summary: 'List every product',
  functionName: 'GetAllProducts',
});

// ---------------------------------------------------------------------------
// Single record lookups
// ---------------------------------------------------------------------------

registerLookup({
  name: 'merchant',
  usage: 'merchant <merchantId>',
  summary: 'Read one merchant',
  functionName: 'ReadMerchant',
  argumentName: 'merchantId',
});

registerLookup({
  name: 'customer',
  usage: 'customer <customerId>',
  summary: 'Read one customer',
  functionName: 'ReadCustomer',
  argumentName: 'customerId',
});

registerLookup({
  name: 'product',
  usage: 'product <productCode>',
  summary: 'Read one product by exact code',
  functionName: 'ReadProduct',
  argumentName: 'productCode',
});

registerLookup({
  name: 'merchant-type',
  usage: 'merchant-type <code>',
  summary: 'Read one merchant type',
  functionName: 'ReadMerchantType',
  argumentName: 'code',
});

registerLookup({
  name: 'invoice',
  usage: 'invoice <invoiceId>',
  summary: 'Read one invoice',
  functionName: 'ReadInvoice',
  argumentName: 'invoiceId',
});


registerLookup({
  name: 'search-name',
  usage: 'search-name <text>',
  summary: 'Search products by name, case insensitive substring',
  functionName: 'SearchProductsByName',
  argumentName: 'text',
});

registerLookup({
  name: 'search-code',
  usage: 'search-code <text>',
  summary: 'Search products by code, substring',
  functionName: 'SearchProductsByCode',
  argumentName: 'text',
});

registerLookup({
  name: 'search-type',
  usage: 'search-type <merchantType>',
  summary: 'Search products by merchant type',
  functionName: 'SearchProductsByMerchantType',
  argumentName: 'merchantType',
});

registerCommand({
  name: 'search-price',
  group: GROUP,
  usage: 'search-price <from> <to>',
  summary: 'Search products by price range, pass 0 as <to> for no upper bound',
  async run({ positional, flags }) {
    const usage = 'search-price <from> <to>';
    const from = requireNumber(requireArg(positional, 0, 'from', usage), 'from');
    const to = requireNumber(requireArg(positional, 1, 'to', usage), 'to');
    return evaluate(flags, 'SearchProductsByPrice', [from, to]);
  },
});

registerLookup({
  name: 'merchant-products',
  usage: 'merchant-products <merchantId>',
  summary: 'List the products one merchant currently offers',
  functionName: 'GetMerchantProducts',
  argumentName: 'merchantId',
});

registerLookup({
  name: 'expiring',
  usage: 'expiring <YYYY-MM-DD>',
  summary: 'Products expiring before a date that are still in stock',
  functionName: 'ProductsExpiringBefore',
  argumentName: 'date',
});

function buildCriteria(flags) {
  const criteria = {};

  if (flags.name !== undefined) criteria.name = String(flags.name);
  if (flags.code !== undefined) criteria.code = String(flags.code);
  if (flags.merchant !== undefined) criteria.merchantId = String(flags.merchant);
  if (flags.type !== undefined) {
    criteria.merchantTypes = String(flags.type).split(',').map((value) => value.trim()).filter(Boolean);
  }
  if (flags['price-from'] !== undefined) {
    criteria.priceFrom = requireNumber(flags['price-from'], 'price-from');
  }
  if (flags['price-to'] !== undefined) {
    criteria.priceTo = requireNumber(flags['price-to'], 'price-to');
  }
  if (flags['expiry-from'] !== undefined) criteria.expiryFrom = String(flags['expiry-from']);
  if (flags['expiry-to'] !== undefined) criteria.expiryTo = String(flags['expiry-to']);
  if (flags['in-stock']) criteria.inStockOnly = true;
  if (flags.sort !== undefined) criteria.sortByPrice = String(flags.sort);

  return criteria;
}

registerCommand({
  name: 'search',
  group: GROUP,
  usage: 'search [--name t] [--code t] [--type A,B] [--merchant id] [--price-from n] [--price-to n] [--expiry-to date] [--in-stock] [--sort asc|desc]',
  summary: 'Combined product search over any mix of the categories',
  async run({ flags }) {
    return evaluate(flags, 'SearchProducts', [JSON.stringify(buildCriteria(flags))]);
  },
});

registerCommand({
  name: 'search-paged',
  group: GROUP,
  usage: 'search-paged [--page-size n] [--bookmark b] [same filters as search]',
  summary: 'Combined product search, one page at a time',
  async run({ flags }) {
    const pageSize = flags['page-size'] === undefined
      ? 5
      : requireInteger(flags['page-size'], 'page-size');
    const bookmark = flags.bookmark === undefined ? '' : String(flags.bookmark);
    return evaluate(flags, 'SearchProductsPaged',
      [JSON.stringify(buildCriteria(flags)), pageSize, bookmark]);
  },
});

registerCommand({
  name: 'invoices',
  group: GROUP,
  usage: 'invoices --customer <id> | --merchant <id> [--min <amount>]',
  summary: 'List the invoices of a customer or a merchant',
  async run({ flags }) {
    if (flags.customer !== undefined) {
      const customerId = String(flags.customer);
      if (flags.min !== undefined) {
        return evaluate(flags, 'CustomerInvoicesAbove',
          [customerId, requireNumber(flags.min, 'min')]);
      }
      return evaluate(flags, 'CustomerInvoices', [customerId]);
    }

    if (flags.merchant !== undefined) {
      if (flags.min !== undefined) {
        throw new Error('--min is only supported together with --customer');
      }
      return evaluate(flags, 'MerchantInvoices', [String(flags.merchant)]);
    }

    throw new Error('pass either --customer <id> or --merchant <id>');
  },
});
