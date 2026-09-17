"""Skill demand analytics, moving average, and linear forecasting pipeline."""

import logging
from typing import Optional

from pydantic import BaseModel, Field

from ai.app.middleware.run_tracker import track_ai_run

logger = logging.getLogger("ai.pipelines.forecasting")


class SkillPostingPoint(BaseModel):
    """Historical posting datapoint for a skill over a specific period."""

    period: str
    job_count: int = Field(default=0, ge=0)
    application_count: int = Field(default=0, ge=0)
    unique_posters: int = Field(default=0, ge=0)


class SkillHistoricalData(BaseModel):
    """Time-series postings data for a single skill."""

    skill: str
    postings: list[SkillPostingPoint] = Field(default_factory=list)


class SkillDemandSnapshot(BaseModel):
    """Aggregated demand snapshot for a skill within a timeframe."""

    skill: str
    period: str
    job_count: int = Field(default=0, ge=0)
    application_count: int = Field(default=0, ge=0)
    unique_posters: int = Field(default=0, ge=0)
    demand_score: float = Field(default=0.0, ge=0.0, le=100.0)
    growth_rate: float = Field(default=0.0)
    app_to_job_ratio: float = Field(default=0.0, ge=0.0)


class SkillDemandForecast(BaseModel):
    """Forecasted future demand metrics for a skill."""

    skill: str
    growth_rate: float
    demand_score: float = Field(ge=0.0, le=100.0)
    projected_30d_demand: float = Field(ge=0.0)


class SkillDemandForecaster:
    """Statistical forecaster and aggregator for skill demand without using LLMs."""

    def __init__(self, db_pool=None) -> None:
        self.db_pool = db_pool
        self.model_name = "linear-demand-forecaster"
        self.model_version = "1.0.0"
        self.pipeline_version = "forecasting-v1"

    def forecast_skill_demand(self, data: SkillHistoricalData) -> SkillDemandForecast:
        """Compute trend and 30-day demand forecast for a skill."""
        postings = data.postings
        n = len(postings)

        if n >= 2:
            x = list(range(n))
            y = [float(p.job_count) for p in postings]
            mean_x = sum(x) / n
            mean_y = sum(y) / n
            denom = sum((xi - mean_x) ** 2 for xi in x)
            slope = (
                sum((xi - mean_x) * (yi - mean_y) for xi, yi in zip(x, y)) / denom
                if denom != 0.0
                else 0.0
            )
            growth_rate = round(slope / max(mean_y, 1.0), 3)
            projected_30d_demand = max(0.0, round(y[-1] + max(slope, 0.0), 1))
        elif n == 1:
            growth_rate = 0.0
            projected_30d_demand = float(postings[0].job_count)
        else:
            growth_rate = 0.0
            projected_30d_demand = 0.0

        total_jobs = sum(p.job_count for p in postings)
        total_apps = sum(p.application_count for p in postings)

        # Demand score normalized [0.0, 100.0] based on jobs, apps, and positive growth
        job_component = min(50.0, total_jobs * 2.5)
        app_component = min(30.0, total_apps * 0.5)
        growth_component = min(20.0, max(0.0, growth_rate * 25.0))
        demand_score = min(100.0, round(job_component + app_component + growth_component, 1))

        return SkillDemandForecast(
            skill=data.skill,
            growth_rate=growth_rate,
            demand_score=demand_score,
            projected_30d_demand=projected_30d_demand,
        )

    def create_snapshot(
        self, data: SkillHistoricalData, period: str = "monthly"
    ) -> SkillDemandSnapshot:
        """Aggregate historical points into a single period snapshot."""
        postings = data.postings
        total_jobs = sum(p.job_count for p in postings)
        total_apps = sum(p.application_count for p in postings)
        unique_posters = max((p.unique_posters for p in postings), default=0)

        app_to_job_ratio = (
            round(total_apps / total_jobs, 2) if total_jobs > 0 else 0.0
        )

        forecast = self.forecast_skill_demand(data)

        return SkillDemandSnapshot(
            skill=data.skill,
            period=period,
            job_count=total_jobs,
            application_count=total_apps,
            unique_posters=unique_posters,
            demand_score=forecast.demand_score,
            growth_rate=forecast.growth_rate,
            app_to_job_ratio=app_to_job_ratio,
        )

    async def analyze_and_record(
        self, period: str, skills_data: list[SkillHistoricalData]
    ) -> tuple[list[SkillDemandSnapshot], list[SkillDemandForecast]]:
        """Forecast and snapshot skill demand, record run, and persist to database."""
        async with track_ai_run(
            feature="skill_demand_forecasting",
            entity_type="skills",
            entity_id=period,
            model_name=self.model_name,
            model_version=self.model_version,
            pipeline_version=self.pipeline_version,
            input_data={
                "period": period,
                "skills_count": len(skills_data),
            },
            db_pool=self.db_pool,
        ) as tracker:
            snapshots: list[SkillDemandSnapshot] = []
            forecasts: list[SkillDemandForecast] = []

            for data in skills_data:
                snapshot = self.create_snapshot(data, period=period)
                forecast = self.forecast_skill_demand(data)
                snapshots.append(snapshot)
                forecasts.append(forecast)

            tracker.set_output(
                {
                    "period": period,
                    "snapshots_count": len(snapshots),
                    "forecasts_count": len(forecasts),
                }
            )
            tracker.set_confidence(1.0)

            if self.db_pool is not None:
                try:
                    async with self.db_pool.acquire() as conn:
                        for s in snapshots:
                            # Upsert snapshot if skill exists in canonical skills table
                            await conn.execute(
                                """
                                INSERT INTO skill_demand_snapshots (
                                    skill_id, period, job_count, application_count, unique_posters, demand_score, growth_rate
                                )
                                SELECT id, $2, $3, $4, $5, $6, $7
                                FROM skills
                                WHERE canonical_name = $1
                                ON CONFLICT (skill_id, period) DO UPDATE SET
                                    job_count = EXCLUDED.job_count,
                                    application_count = EXCLUDED.application_count,
                                    unique_posters = EXCLUDED.unique_posters,
                                    demand_score = EXCLUDED.demand_score,
                                    growth_rate = EXCLUDED.growth_rate,
                                    created_at = NOW();
                                """,
                                s.skill,
                                s.period,
                                s.job_count,
                                s.application_count,
                                s.unique_posters,
                                s.demand_score,
                                s.growth_rate,
                            )
                except Exception as exc:
                    logger.warning(
                        "Failed to persist skill demand snapshots to PostgreSQL: %s", exc
                    )

            return snapshots, forecasts
