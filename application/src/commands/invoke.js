import { readFileSync } from 'node:fs';

import { invoke, withGateway } from '../gateway.js';
import {
  connectionOptions, registerCommand, requireArg, requireInteger, requireNumber,
} from '../cli.js';

const GROUP = 'Invoke (changes the ledger)';

function submit(flags, functionName, args) {
  return withGateway(connectionOptions(flags), (connection) =>
    invoke(connection, functionName, args));
}


function readJsonArray(positional, flags, name) {
  const raw = flags.file
    ? readFileSync(String(flags.file), 'utf8')
    : positional[0];

  if (!raw) {
    throw new Error(`provide the ${name} as a JSON array, either inline or with --file <path>`);
  }

  let parsed;
  try {
    parsed = JSON.parse(raw);
  } catch (error) {
    throw new Error(`${name} is not valid JSON: ${error.message}`);
  }
  if (!Array.isArray(parsed)) {
    throw new Error(`${name} must be a JSON array, got ${typeof parsed}`);
  }
  return JSON.stringify(parsed);
}


registerCommand({
  name: 'create-merchant-type',
  group: GROUP,
  usage: 'create-merchant-type <code> <name> [description]',
  summary: 'Add a line of business to the merchant type catalogue',
  async run({ positional, flags }) {
    const code = requireArg(positional, 0, 'code', 'create-merchant-type <code> <name> [description]');
    const name = requireArg(positional, 1, 'name', 'create-merchant-type <code> <name> [description]');
    return submit(flags, 'CreateMerchantType', [code, name, positional[2] ?? '']);
  },
});


registerCommand({
  name: 'create-merchant',
  group: GROUP,
  usage: 'create-merchant <id> <name> <type> <taxId> [openingBalance]',
  summary: 'Register a new merchant',
  async run({ positional, flags }) {
    const usage = 'create-merchant <id> <name> <type> <taxId> [openingBalance]';
    const id = requireArg(positional, 0, 'id', usage);
    const name = requireArg(positional, 1, 'name', usage);
    const type = requireArg(positional, 2, 'type', usage);
    const taxId = requireArg(positional, 3, 'taxId', usage);
    const balance = positional[4] === undefined ? 0 : requireNumber(positional[4], 'openingBalance');
    return submit(flags, 'CreateMerchant', [id, name, type, taxId, balance]);
  },
});

registerCommand({
  name: 'create-merchants',
  group: GROUP,
  usage: 'create-merchants --file <merchants.json>',
  summary: 'Register several merchants from a JSON array, all or nothing',
  async run({ positional, flags }) {
    return submit(flags, 'CreateMerchants', [readJsonArray(positional, flags, 'the merchant list')]);
  },
});

registerCommand({
  name: 'change-merchant-type',
  group: GROUP,
  usage: 'change-merchant-type <merchantId> <newType>',
  summary: 'Change a merchant line of business and propagate it to its products',
  async run({ positional, flags }) {
    const usage = 'change-merchant-type <merchantId> <newType>';
    const merchantId = requireArg(positional, 0, 'merchantId', usage);
    const newType = requireArg(positional, 1, 'newType', usage);
    return submit(flags, 'ChangeMerchantType', [merchantId, newType]);
  },
});


registerCommand({
  name: 'create-customer',
  group: GROUP,
  usage: 'create-customer <id> <firstName> <lastName> <email> [openingBalance]',
  summary: 'Register a new customer',
  async run({ positional, flags }) {
    const usage = 'create-customer <id> <firstName> <lastName> <email> [openingBalance]';
    const id = requireArg(positional, 0, 'id', usage);
    const firstName = requireArg(positional, 1, 'firstName', usage);
    const lastName = requireArg(positional, 2, 'lastName', usage);
    const email = requireArg(positional, 3, 'email', usage);
    const balance = positional[4] === undefined ? 0 : requireNumber(positional[4], 'openingBalance');
    return submit(flags, 'CreateCustomer', [id, firstName, lastName, email, balance]);
  },
});

registerCommand({
  name: 'create-customers',
  group: GROUP,
  usage: 'create-customers --file <customers.json>',
  summary: 'Register several customers from a JSON array, all or nothing',
  async run({ positional, flags }) {
    return submit(flags, 'CreateCustomers', [readJsonArray(positional, flags, 'the customer list')]);
  },
});


registerCommand({
  name: 'add-product',
  group: GROUP,
  usage: 'add-product <merchantId> <code> <name> <price> <quantity> [expiryDate]',
  summary: 'Add one product to a merchant offering',
  async run({ positional, flags }) {
    const usage = 'add-product <merchantId> <code> <name> <price> <quantity> [expiryDate]';
    const merchantId = requireArg(positional, 0, 'merchantId', usage);
    const code = requireArg(positional, 1, 'code', usage);
    const name = requireArg(positional, 2, 'name', usage);
    const price = requireNumber(requireArg(positional, 3, 'price', usage), 'price');
    const quantity = requireInteger(requireArg(positional, 4, 'quantity', usage), 'quantity');
    // The chaincode signature puts the expiry date before price and quantity.
    return submit(flags, 'AddProduct',
      [merchantId, code, name, positional[5] ?? '', price, quantity]);
  },
});

registerCommand({
  name: 'add-products',
  group: GROUP,
  usage: 'add-products <merchantId> --file <products.json>',
  summary: 'Add several products to one merchant, all or nothing',
  async run({ positional, flags }) {
    const merchantId = requireArg(positional, 0, 'merchantId',
      'add-products <merchantId> --file <products.json>');
    const list = readJsonArray(positional.slice(1), flags, 'the product list');
    return submit(flags, 'AddProducts', [merchantId, list]);
  },
});

registerCommand({
  name: 'update-price',
  group: GROUP,
  usage: 'update-price <productCode> <newPrice>',
  summary: 'Change the price of a product',
  async run({ positional, flags }) {
    const usage = 'update-price <productCode> <newPrice>';
    const code = requireArg(positional, 0, 'productCode', usage);
    const price = requireNumber(requireArg(positional, 1, 'newPrice', usage), 'newPrice');
    return submit(flags, 'UpdatePrice', [code, price]);
  },
});

registerCommand({
  name: 'restock',
  group: GROUP,
  usage: 'restock <productCode> <additionalQuantity>',
  summary: 'Increase the stock of a product',
  async run({ positional, flags }) {
    const usage = 'restock <productCode> <additionalQuantity>';
    const code = requireArg(positional, 0, 'productCode', usage);
    const quantity = requireInteger(
      requireArg(positional, 1, 'additionalQuantity', usage), 'additionalQuantity');
    return submit(flags, 'RestockProduct', [code, quantity]);
  },
});


registerCommand({
  name: 'buy',
  group: GROUP,
  usage: 'buy <customerId> <merchantId> <productCode> [quantity]',
  summary: 'Buy a product, which moves money and issues an invoice',
  async run({ positional, flags }) {
    const usage = 'buy <customerId> <merchantId> <productCode> [quantity]';
    const customerId = requireArg(positional, 0, 'customerId', usage);
    const merchantId = requireArg(positional, 1, 'merchantId', usage);
    const productCode = requireArg(positional, 2, 'productCode', usage);
    const quantity = positional[3] === undefined
      ? 1
      : requireInteger(positional[3], 'quantity');
    return submit(flags, 'BuyProduct', [customerId, merchantId, productCode, quantity]);
  },
});

registerCommand({
  name: 'deposit',
  group: GROUP,
  usage: 'deposit <merchant|customer> <id> <amount>',
  summary: 'Pay money into a merchant or customer account',
  async run({ positional, flags }) {
    const usage = 'deposit <merchant|customer> <id> <amount>';
    const entityType = requireArg(positional, 0, 'merchant|customer', usage);
    const id = requireArg(positional, 1, 'id', usage);
    const amount = requireNumber(requireArg(positional, 2, 'amount', usage), 'amount');
    return submit(flags, 'Deposit', [entityType, id, amount]);
  },
});


registerCommand({
  name: 'init-ledger',
  group: GROUP,
  usage: 'init-ledger [--channel <name>]',
  summary: 'Write the initial state, skipping records that already exist',
  async run({ flags }) {
    return submit(flags, 'InitLedger', []);
  },
});
