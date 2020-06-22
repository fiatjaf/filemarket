CREATE TABLE users (
  id text PRIMARY KEY,
  wallet_key text
);

CREATE TABLE files (
  id text PRIMARY KEY,
  seller text NOT NULL REFERENCES users(id),
  price_msat numeric(13) NOT NULL,
  metadata jsonb NOT NULL,
  torrent bytea NOT NULL
);

CREATE TABLE sales (
  id text PRIMARY KEY, -- the lntx id from lnpay, minus the "lntx_" prefix
  file_id text NOT NULL REFERENCES files(id),
  status text NOT NULL DEFAULT 'pending' -- when "canceled" the buyer can redeem this
      CHECK (status IN ('pending', 'fulfilled', 'canceled'))
);
