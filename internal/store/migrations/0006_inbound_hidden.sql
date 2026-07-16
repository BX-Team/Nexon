-- Hidden inbounds are still provisioned onto their node (users get pushed), but
-- are excluded from generated subscriptions. Lets a node act as a private chain
-- exit whose credentials never appear in a client's subscription.
ALTER TABLE inbounds ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0;
