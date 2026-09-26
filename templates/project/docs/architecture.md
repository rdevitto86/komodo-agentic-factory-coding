# Architecture — <Repo Name>

The parts and how they connect: small enough for a planner and a reviewer to read whole. Names and reasons only; a flag, field, or version belongs in `system-design.md`, and a number in `prd.md`. Every heading here exists in exactly one spec file, so a task cites one place.

## Purpose

<What the system does and for whom, in one paragraph, and the principles that shape it.>

## Context

<The people and external systems it works with, one line each. How each integration works lives in system-design.md.>

## Components

<One line per component: name, responsibility, where it runs.>

## Boundaries

<What is inside the system, what it depends on, what it never does, and where trust changes.>

## Data flow

<How a request or an event moves through the components, start to finish.>

## Glossary

<One line per term this repo uses in a specific sense.>
