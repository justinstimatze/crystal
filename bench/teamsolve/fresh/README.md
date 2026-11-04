# Fresh problems with judge-trusted hidden tests

LiveCodeBench's public data ends in April 2025, before current models' training cutoffs. This builds a
fresh set from recent AtCoder contests:

1. `atc_scrape.py OUT contest...`: English statement, samples and editorial code per task (1 request/s;
   pages are parsed, never executed). AtCoder submissions now need a login, so editorials are the
   source of reference solutions.
2. `write_gens.py SRC GENS`: Claude Opus 5.5 writes one seeded input generator per task from the
   statement (model calls only).
3. `modal run modal_build.py --src SRC --gens GENS --acl - --out X`: on Modal, in a container with no
   network: compile the editorial references (AtCoder Library cloned at image build), keep those that
   pass every sample, run the generator for 20 small + 10 large seeds, keep an input only if every
   reference ends cleanly and all agree; write each task to the `crystal-teamsolve` volume
   (`/freshbuild/`). External and model-written code never runs locally. Large results go through the
   volume because Modal's direct blob transfer bypasses the session proxy.

2026-11-04 build: contests abc476-482 and arc230-233 (Sep-Oct 2026), 69 tasks scraped, 39 with
editorial code, 34 built (5 dropped: references that do not compile or fail the samples; no ARC task
had editorial code). Every reference agreed on every generated input. The generator author being a
frontier model affects only which inputs are drawn, never the expected outputs.
