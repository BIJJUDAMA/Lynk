-- Seed initial campus opportunities posted by poster@campus.edu
INSERT INTO jobs (
    id,
    created_by,
    title,
    description,
    required_skills,
    department,
    deadline,
    status,
    created_at,
    updated_at
)
VALUES
    (
        'c1000000-0000-0000-0000-000000000001',
        'a2be7abb-a253-4b8d-b3c3-5a5f39416239',
        'Distributed Consensus Test Harness',
        'Build an automated failure injection harness for evaluating Raft consensus edge cases under network partitions.',
        ARRAY['Go', 'Raft', 'Docker'],
        'Computer Science',
        CURRENT_DATE + INTERVAL '30 days',
        'open',
        NOW(),
        NOW()
    ),
    (
        'c1000000-0000-0000-0000-000000000002',
        'a2be7abb-a253-4b8d-b3c3-5a5f39416239',
        'Campus Autonomous Shuttle UI & Telemetry',
        'Develop real-time sensor visualization dashboards and teleoperation controls for student-built autonomous shuttles.',
        ARRAY['React', 'TypeScript', 'WebSocket'],
        'Robotics',
        CURRENT_DATE + INTERVAL '45 days',
        'open',
        NOW(),
        NOW()
    ),
    (
        'c1000000-0000-0000-0000-000000000003',
        'a2be7abb-a253-4b8d-b3c3-5a5f39416239',
        'Bioinformatics Sequence Alignment Pipeline',
        'Implement high-throughput variant calling and multiple sequence alignment pipelines for genomic research lab.',
        ARRAY['Python', 'Bioinformatics', 'PostgreSQL'],
        'Bioengineering',
        CURRENT_DATE + INTERVAL '60 days',
        'open',
        NOW(),
        NOW()
    )
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    required_skills = EXCLUDED.required_skills,
    department = EXCLUDED.department,
    deadline = EXCLUDED.deadline,
    status = EXCLUDED.status,
    updated_at = NOW();
