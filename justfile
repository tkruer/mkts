set shell := ["bash", "-cu"]

default: build

build:
    cargo build

release:
    cargo build --release

run *args:
    cargo run -p mkts -- {{args}}

watch *args:
    cargo watch -x "run -p mkts -- {{args}}"

fmt:
    cargo fmt

lint:
    cargo clippy --all-targets --all-features -D warnings

check:
    cargo check --all-targets

test:
    cargo test

fix:
    cargo clippy --fix --allow-dirty --allow-staged

clean:
    cargo clean

update:
    cargo update

tree:
    cargo tree

dev symbol="VOO":
    cargo run -p mkts -- {{symbol}}

release-run symbol="VOO":
    cargo run -p mkts --release -- {{symbol}}

install:
    cargo install --path crates/app --force
