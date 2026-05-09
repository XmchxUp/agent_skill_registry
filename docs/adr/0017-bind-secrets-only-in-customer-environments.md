# Bind Secrets only in customer environments

Skills declare Secret References but never carry secret values or customer-specific credential configuration. ADP can validate and simulate required permissions, while the customer-environment Controller resolves and projects secrets according to local policy, preventing ADP and exported artifacts from becoming customer secret holders.
