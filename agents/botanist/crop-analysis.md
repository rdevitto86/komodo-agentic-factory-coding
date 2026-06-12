# Skill: /crop-analysis

Analyze crop health, yield data, or growing conditions and produce an actionable field report.

## Usage

```
/crop-analysis <crop> [--health|--yield|--soil|--full]
```

- `<crop>` — Crop type, e.g. `wheat`, `tomato`, `soybean`, `leafy-greens`.
- `--health` — Focus on disease, pest, and nutrient deficiency diagnosis.
- `--yield` — Analyze yield data, identify limiting factors, and recommend improvements.
- `--soil` — Interpret soil test results and generate an amendment plan.
- `--full` — Comprehensive report covering health, yield, and soil (default if no flag).

---

## Before generating anything

Ask the user for:
1. Growing region and current season / growth stage.
2. Observed symptoms (if `--health`): affected area percentage, symptom description, onset timing.
3. Yield data (if `--yield`): current yield vs. historical average or target.
4. Soil test results (if `--soil`): pH, macro/micronutrient levels, organic matter percentage, CEC.
5. Recent inputs: fertilizer applications, pesticide/herbicide treatments, irrigation schedule.
6. Weather anomalies in the past 4–8 weeks (drought, flooding, frost, heat stress).

---

## Report sections

### Executive summary
One paragraph: current crop status, primary limiting factor, and top recommended action.

### Diagnosis (--health)
- Most likely cause(s) ranked by probability
- Supporting evidence (symptoms, conditions, timing)
- Differential diagnoses ruled out
- Recommended confirmation steps (scouting protocol, lab test, or visual guide reference)

### Yield analysis (--yield)
- Actual vs. expected yield gap in absolute and percentage terms
- Identified limiting factors (water, nutrients, pest pressure, variety, planting density)
- Prioritized improvement actions with estimated yield impact

### Soil amendment plan (--soil)
- pH adjustment recommendation (lime or sulfur rate, timing)
- Macronutrient recommendations (N-P-K rates, form, application timing)
- Micronutrient corrections where deficient
- Organic matter improvement if below target

### Treatment recommendations
| Priority | Action | Product / Rate | Timing | Notes |
|----------|--------|---------------|--------|-------|
| 1 | ... | ... | ... | ... |

### Monitoring plan
What to observe, when to re-scout, and trigger thresholds for escalating treatment.

---

## After generating

Remind the user that:
- Field variability means recommendations should be validated by in-person scouting before large-scale application.
- Pesticide/herbicide labels are legal documents — always follow label rates and re-entry intervals.
- Consult a certified agronomist or extension service for complex or high-value situations.
