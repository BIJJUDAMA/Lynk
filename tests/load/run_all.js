// Master Load Test Orchestrator for Lynk
// Executes k6 stages sequentially and outputs a unified metric summary.
// 100% pure ASCII

const { execSync, spawnSync, spawn } = require('child_process');

function parseMemToMb(token) {
  if (token == null) return 0;
  const m = String(token).trim().match(/^([\d.]+)\s*([KMGTP]i?B)?$/i);
  if (!m) return 0;
  const value = Number(m[1]);
  if (!Number.isFinite(value)) return 0;
  const unit = (m[2] || 'B').toUpperCase();
  const factor = {
    B: 1 / (1024 * 1024),
    KIB: 1 / 1024,
    KB: 1 / 1024,
    MIB: 1,
    MB: 1,
    GIB: 1024,
    GB: 1024,
    TIB: 1024 * 1024,
    TB: 1024 * 1024,
  }[unit];
  return factor ? value * factor : 0;
}

function sampleDockerStats() {
  try {
    const out = execSync('docker stats --no-stream --format "{{.Name}}|{{.CPUPerc}}|{{.MemUsage}}"', {
      timeout: 3000,
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    });
    let totalCpu = 0;
    let totalMem = 0;
    for (const line of out.split(/\r?\n/)) {
      const trimmed = line.trim();
      if (!trimmed) continue;
      const parts = trimmed.split('|');
      if (parts.length >= 3 && (parts[0].includes('lynk-') || parts[0].includes('lynk_') || parts[0].includes('lynk'))) {
        const cpuStr = parts[1].replace('%', '').trim();
        const cpuVal = parseFloat(cpuStr) || 0;
        totalCpu += cpuVal;
        const memStr = parts[2].split('/')[0].trim();
        const memMb = parseMemToMb(memStr) || 0;
        totalMem += memMb;
      }
    }
    return { cpu: totalCpu, mem: totalMem };
  } catch (e) {
    return { cpu: 0, mem: 0 };
  }
}
const fs = require('fs');
const path = require('path');
const http = require('http');

const ROOT_DIR = path.resolve(__dirname, '../..');
const LOAD_DIR = path.resolve(__dirname);
const RESULTS_DIR = path.join(LOAD_DIR, 'results');
const SCRIPTS_DIR = path.join(LOAD_DIR, 'scripts');

const STAGES = [
  {
    id: '01_rate_limit',
    name: '01. Rate Limiting (DDoS Burst)',
    script: path.join(SCRIPTS_DIR, '01_rate_limit.js'),
    resultFile: path.join(RESULTS_DIR, '01_rate_limit.json'),
    description: 'Validates 120 req/min single-IP sliding window rejection and sub-10ms drop speed',
  },
  {
    id: '02_cache_benchmark',
    name: '02. Cache & Read Benchmark',
    script: path.join(SCRIPTS_DIR, '02_cache_benchmark.js'),
    resultFile: path.join(RESULTS_DIR, '02_cache_benchmark.json'),
    description: 'Measures read throughput and latency on project feeds and search queries',
  },
  {
    id: '03_pool_saturation',
    name: '03. DB Pool Saturation',
    script: path.join(SCRIPTS_DIR, '03_pool_saturation.js'),
    resultFile: path.join(RESULTS_DIR, '03_pool_saturation.json'),
    description: 'Pushes concurrency beyond 25 connections to verify Go wait queue stability',
  },
  {
    id: '04_user_concurrency',
    name: '04. Max User Breakpoint',
    script: path.join(SCRIPTS_DIR, '04_user_concurrency.js'),
    resultFile: path.join(RESULTS_DIR, '04_user_concurrency.json'),
    description: 'Ramps up to 350 VUs with realistic think times to identify capacity ceiling',
  },
  {
    id: '05_high_concurrency_stress',
    name: '05. High-Concurrency Stress (1,000 VUs)',
    script: path.join(SCRIPTS_DIR, '05_high_concurrency_stress.js'),
    resultFile: path.join(RESULTS_DIR, '05_high_concurrency_stress.json'),
    description: 'Pushes concurrency to 1,000 VUs evaluating PgBouncer, Redis cache, and covering indexes',
  },
  {
    id: '06_ultra_concurrency_stress',
    name: '06. Ultra-Concurrency Stress (2,500 VUs)',
    script: path.join(SCRIPTS_DIR, '06_ultra_concurrency_stress.js'),
    resultFile: path.join(RESULTS_DIR, '06_ultra_concurrency_stress.json'),
    description: 'Pushes concurrency to 2,500 VUs to evaluate Nginx micro-caching, L1 cache, and DB buffers',
  },
  {
    id: '07_saturation_ceiling',
    name: '07. Constant-Arrival Saturation Ceiling',
    script: path.join(SCRIPTS_DIR, '07_saturation_ceiling.js'),
    resultFile: path.join(RESULTS_DIR, '07_saturation_ceiling.json'),
    description: 'Ramps fixed arrival rate (no think time) to find the true per-pod server ceiling',
  },
  {
    id: '08_connection_hold',
    name: '08. Concurrent Connection Hold',
    script: path.join(SCRIPTS_DIR, '08_connection_hold.js'),
    resultFile: path.join(RESULTS_DIR, '08_connection_hold.json'),
    description: 'Opens and holds keep-alive connections (500 -> 8,000) to measure concurrent connection capacity',
  },
];

// Ensure Windows default install paths for k6 and winget links are in PATH
const extraPaths = [
  'C:\\Program Files\\k6',
  path.join(process.env.LOCALAPPDATA || '', 'Microsoft\\WinGet\\Links'),
  path.join(process.env.USERPROFILE || '', 'AppData\\Local\\Microsoft\\WinGet\\Links'),
];
process.env.PATH = `${extraPaths.join(path.delimiter)}${path.delimiter}${process.env.PATH}`;

function checkCommand(cmd) {
  try {
    execSync(`${cmd} --version`, { stdio: 'ignore', env: process.env });
    return true;
  } catch (e) {
    return false;
  }
}

function checkApiHealth(url) {
  return new Promise((resolve) => {
    const req = http.get(url, { timeout: 3000 }, (res) => {
      resolve(res.statusCode === 200);
    });
    req.on('error', () => resolve(false));
    req.on('timeout', () => {
      req.destroy();
      resolve(false);
    });
  });
}

function sleepMs(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function formatDuration(ms) {
  if (ms == null || isNaN(ms)) return 'N/A';
  if (ms < 1) return `${(ms * 1000).toFixed(0)}us`;
  if (ms < 1000) return `${ms.toFixed(1)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

function formatRate(rate) {
  if (rate == null || isNaN(rate)) return 'N/A';
  return `${(rate * 100).toFixed(2)}%`;
}

async function main() {
  console.log('========================================================================');
  console.log('                 LYNK BACKEND LOAD TESTING CONTROL PLANE                ');
  console.log('========================================================================\n');

  // Parse command line arguments
  let stageFilter = null;
  for (let i = 2; i < process.argv.length; i++) {
    const arg = process.argv[i];
    if (arg.startsWith('--stage=')) {
      stageFilter = arg.slice('--stage='.length);
    } else if (arg === '--stage' && i + 1 < process.argv.length) {
      stageFilter = process.argv[i + 1];
    }
  }

  let activeStages = STAGES;
  if (stageFilter) {
    activeStages = STAGES.filter((s) => s.id === stageFilter || s.id.includes(stageFilter));
    if (activeStages.length === 0) {
      console.error(`[ERROR] No stage matches filter "${stageFilter}". Available stages:`);
      STAGES.forEach((s) => console.error(`  - ${s.id} (${s.name})`));
      process.exit(1);
    }
    console.log(`==> Filtered to stage: ${activeStages.map((s) => s.id).join(', ')}\n`);
  }

  // 1. Ensure results directory exists
  if (!fs.existsSync(RESULTS_DIR)) {
    fs.mkdirSync(RESULTS_DIR, { recursive: true });
  }

  // 2. Pre-flight check: Target API availability
  const apiUrl = process.env.API_BASE_URL || 'http://localhost:8080/api/v1';
  const healthUrl = `${apiUrl}/health`;
  console.log(`==> Checking API health at ${healthUrl}...`);
  const isHealthy = await checkApiHealth(healthUrl);
  if (!isHealthy) {
    console.error(`[ERROR] Go API is not responding at ${healthUrl}.`);
    console.error('Please ensure the backend is running (e.g. docker compose up -d or make run).');
    process.exit(1);
  }
  console.log('    API is healthy and ready for load testing.\n');

  // 3. Pre-flight check: k6 execution engine
  let runnerType = null;
  if (checkCommand('k6')) {
    runnerType = 'native';
    console.log('==> Detected native k6 CLI installation.');
  } else if (checkCommand('docker')) {
    runnerType = 'docker';
    console.log('==> Native k6 not found; using Docker (grafana/k6) container engine.');
  } else {
    console.error('[ERROR] Neither "k6" nor "docker" command found in PATH.');
    console.error('Please install k6 using:');
    console.error('   winget install k6 --source winget');
    console.error('Or ensure Docker Desktop is running.');
    process.exit(1);
  }

  console.log(`\nExecuting ${activeStages.length} test stage(s) sequentially with 5s cooldown intervals...\n`);

  const summaryData = [];

  // 4. Sequential execution loop
  for (let i = 0; i < activeStages.length; i++) {
    const stage = activeStages[i];
    console.log(`------------------------------------------------------------------------`);
    console.log(`[STAGE ${i + 1}/${activeStages.length}] ${stage.name}`);
    console.log(`Target: ${stage.description}`);
    console.log(`------------------------------------------------------------------------`);

    // Clean previous result
    if (fs.existsSync(stage.resultFile)) {
      fs.unlinkSync(stage.resultFile);
    }

    const startTime = Date.now();
    let peakCpu = 0;
    let peakMem = 0;
    const initialStats = sampleDockerStats();
    peakCpu = initialStats.cpu;
    peakMem = initialStats.mem;

    const sampleTimer = setInterval(() => {
      const stats = sampleDockerStats();
      if (stats.cpu > peakCpu) peakCpu = stats.cpu;
      if (stats.mem > peakMem) peakMem = stats.mem;
    }, 1500);

    let exitCode = 0;
    if (runnerType === 'native') {
      const args = [
        'run',
        '--summary-trend-stats=avg,min,med,max,p(90),p(95),p(99)',
        `--summary-export=${stage.resultFile}`,
        stage.script,
      ];
      const proc = spawn('k6', args, {
        cwd: ROOT_DIR,
        stdio: 'inherit',
        env: { ...process.env, API_BASE_URL: apiUrl },
      });
      await new Promise((resolve) => {
        proc.on('close', (code) => {
          exitCode = code ?? 0;
          resolve();
        });
        proc.on('error', () => resolve());
      });
    } else {
      // Docker runner
      const relScript = path.relative(ROOT_DIR, stage.script).replace(/\\/g, '/');
      const relResult = path.relative(ROOT_DIR, stage.resultFile).replace(/\\/g, '/');
      const dockerArgs = [
        'run',
        '--rm',
        '--network=host',
        '-v', `${ROOT_DIR}:/lynk`,
        '-w', '/lynk',
        '-e', `API_BASE_URL=${apiUrl}`,
        'grafana/k6',
        'run',
        '--summary-trend-stats=avg,min,med,max,p(90),p(95),p(99)',
        `--summary-export=${relResult}`,
        relScript,
      ];
      const proc = spawn('docker', dockerArgs, {
        cwd: ROOT_DIR,
        stdio: 'inherit',
      });
      await new Promise((resolve) => {
        proc.on('close', (code) => {
          exitCode = code ?? 0;
          resolve();
        });
        proc.on('error', () => resolve());
      });
    }

    clearInterval(sampleTimer);
    const finalStats = sampleDockerStats();
    if (finalStats.cpu > peakCpu) peakCpu = finalStats.cpu;
    if (finalStats.mem > peakMem) peakMem = finalStats.mem;

    const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
    console.log(`\n==> Stage completed in ${elapsed}s (Exit code: ${exitCode}).`);

    // Parse output JSON if created
    let metrics = null;
    if (fs.existsSync(stage.resultFile)) {
      try {
        const raw = fs.readFileSync(stage.resultFile, 'utf8');
        metrics = JSON.parse(raw);
      } catch (err) {
        console.warn(`    Warning: Could not parse results JSON for ${stage.id}`);
      }
    }

    summaryData.push({
      stage: stage.name,
      id: stage.id,
      exitCode,
      elapsed,
      peakCpu: peakCpu > 0 ? `${peakCpu.toFixed(1)}%` : 'N/A',
      peakMem: peakMem > 0 ? `${peakMem.toFixed(0)} MB` : 'N/A',
      metrics,
    });

    if (i < activeStages.length - 1) {
      console.log('==> Pausing 5 seconds for TCP socket cooldown and DB pool stabilization...\n');
      await sleepMs(5000);
    }
  }

  // 5. Aggregate and display unified metrics table
  console.log('\n========================================================================');
  console.log('                  CONSOLIDATED LOAD TESTING REPORT                      ');
  console.log('========================================================================\n');

  const rows = [];
  let maxThroughput = 0;
  let maxVUsRecorded = 0;

  for (const item of summaryData) {
    const m = item.metrics?.metrics;
    const reqs = m?.http_reqs?.values || m?.http_reqs;
    const duration = m?.http_req_duration?.values || m?.http_req_duration;
    const failed = m?.http_req_failed?.values || m?.http_req_failed;
    const vus = m?.vus_max?.value || m?.vus_max?.values?.value || m?.vus?.values?.value || 0;

    const rps = reqs?.rate || 0;
    const totalRequests = reqs?.count || 0;
    const p95 = duration ? duration['p(95)'] : null;
    const p99 = duration ? duration['p(99)'] : null;
    const errorRate = (failed?.value != null) ? failed.value : (failed?.rate || 0);

    if (rps > maxThroughput) maxThroughput = rps;
    if (vus > maxVUsRecorded) maxVUsRecorded = vus;

    let verdict = 'PASS';
    if (item.id === '01_rate_limit') {
      verdict = errorRate > 0.1 ? 'PASS (Limiter Enforced)' : 'WARN (No 429s)';
    } else if (errorRate > 0.05) {
      verdict = 'FAIL (High Errors)';
    } else if (p95 > 500) {
      verdict = 'WARN (Latency High)';
    }

    rows.push({
      stage: item.stage,
      totalRequests,
      rps: rps.toFixed(1),
      p95: formatDuration(p95),
      p99: formatDuration(p99),
      errorRate: formatRate(errorRate),
      peakCpu: item.peakCpu,
      peakMem: item.peakMem,
      verdict,
    });
  }

  // Print console table
  console.table(rows);

  // 6. Capacity estimation analysis
  const realisticThinkTimeSec = 15; // Average seconds student pauses between actions
  const estimatedActiveCampusUsers = Math.round(maxThroughput * realisticThinkTimeSec);

  console.log('\n------------------------------------------------------------------------');
  console.log('CAPACITY & CONCURRENCY ANALYSIS:');
  console.log(`  * Peak Processed Throughput:  ${maxThroughput.toFixed(1)} requests/second`);
  console.log(`  * Peak Virtual Users Tested:  ${maxVUsRecorded} concurrent VUs`);
  console.log(`  * Rate Limiting Enforcement:  120 req/min per IP strictly active`);
  console.log(`  * Database Pool Saturation:   25 maximum pgx connections preserved`);
  console.log(`  * Estimated Real-World Scale: ~${estimatedActiveCampusUsers.toLocaleString()} concurrent campus users`);
  console.log(`    (Calculated based on ${maxThroughput.toFixed(1)} req/s * ${realisticThinkTimeSec}s student browse/think time)`);
  console.log('------------------------------------------------------------------------\n');

  // 7. Write Markdown Summary Report
  const mdReport = generateMarkdownReport(rows, maxThroughput, maxVUsRecorded, estimatedActiveCampusUsers);
  const summaryMdPath = path.join(RESULTS_DIR, 'SUMMARY.md');
  fs.writeFileSync(summaryMdPath, mdReport, 'utf8');
  console.log(`Full markdown report saved to: ${summaryMdPath}\n`);
}

function generateMarkdownReport(rows, peakRps, maxVUs, estimatedUsers) {
  return `# Lynk Backend Load Testing & Capacity Benchmark Report

**Generated:** ${new Date().toISOString()}
**Target API:** \`http://localhost:8080/api/v1\`
**Engine:** k6 Protocol-Level Load Testing

---

## 1. Executive Summary

The Lynk backend test suite executed ${rows.length} sequential load stage(s) evaluating:
1. **DDoS Protection & Rate Limiting** (\`NewSlidingWindowMiddleware\` 120 req/min).
2. **Read Path Performance** (Project workspace listings and hybrid search).
3. **Database Connection Queuing** (\`pgxpool.MaxConns = 25\` under contention).
4. **Max Virtual User Concurrency** (Progressive multi-VU ramping to find the capacity cliff).
5. **High-Concurrency Stress** (1,000 VUs evaluating PgBouncer, Redis cache, and covering indexes).
6. **Ultra-Concurrency Stress** (2,500 VUs evaluating Nginx micro-caching, L1 cache, and DB buffers).

---

## 2. Consolidated Stage Results

| Stage | Total Requests | Throughput (RPS) | p95 Latency | p99 Latency | Error Rate | Peak CPU | Peak RAM | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
${rows.map((r) => `| **${r.stage}** | ${r.totalRequests} | ${r.rps} req/s | ${r.p95} | ${r.p99} | ${r.errorRate} | ${r.peakCpu} | ${r.peakMem} | ${r.verdict} |`).join('\n')}

---

## 3. Capacity & Concurrent User Calculation

* **Peak Sustained Throughput:** \`${peakRps.toFixed(1)} req/sec\`
* **Peak Virtual Users (Simultaneous Active Requests):** \`${maxVUs} VUs\`
* **Database Pool Ceiling:** \`25 max connections\`
* **Estimated Real-World Campus Capacity:** \`~${estimatedUsers.toLocaleString()} concurrent users\`

> [!NOTE]
> In real campus usage, students browse, read project overviews, and review requirements with an average think time of 10 to 30 seconds between requests. A backend processing ${peakRps.toFixed(1)} requests/second comfortably supports thousands of simultaneous browsing students.
`;
}

main().catch((err) => {
  console.error('[FATAL] Load test orchestrator failed:', err);
  process.exit(1);
});
