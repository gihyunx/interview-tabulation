# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-11-15

### Added

- Log in as an admin or a judge, each with their own pages.
- Admins can manage judges and other admin accounts, including resetting passwords.
- Admins can add candidates one by one or import them from a CSV file.
- Every candidate has a number. Admins can renumber candidates to close gaps.
- Admins can build rubrics with their own criteria and maximum points.
- Admins can set up interviews as either one-on-one or panel, assign judges, and reorder them.
- Judges pick a candidate and score them against the interview's rubric, and can edit their scores later.
- Scores page shows every candidate's totals per judge and their final average, updated live as judges submit.
- Sort candidates by final average to see the ranking.
- Export results to an Excel (.xlsx) file, with a summary sheet and one sheet per interview.
