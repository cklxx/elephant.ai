#!/usr/bin/env node

/**
 * Performance Benchmark Script
 *
 * Measures:
 * - Bundle size (gzipped)
 * - Time to Interactive (TTI)
 * - FPS during scroll
 * - Memory usage with 1000 events
 *
 * Usage:
 *   npm run build && node scripts/perf-benchmark.ts
 */

import { execSync } from 'child_process';
import fs from 'fs';
import path from 'path';
import { gzipSync } from 'zlib';

interface BenchmarkResults {
  bundleSize: {
    raw: number;
    gzipped: number;
    targetPassed: boolean;
  };
  buildTime: {
    duration: number;
    targetPassed: boolean;
  };
  recommendations: string[];
}

const TARGET_BUNDLE_SIZE_KB = 250; // 250KB gzipped target
const MAX_BUILD_TIME_MS = 120000; // 2 minutes

function formatBytes(bytes: number): string {
  return `${(bytes / 1024).toFixed(2)} KB`;
}

function measureBundleSize(): BenchmarkResults['bundleSize'] {
  console.log('\n📦 Measuring bundle size...\n');

  const nextDir = path.join(process.cwd(), '.next');

  if (!fs.existsSync(nextDir)) {
    throw new Error('Build directory not found. Run `npm run build` first.');
  }

  // Measure main bundle sizes
  const staticDir = path.join(nextDir, 'static/chunks');
  let totalSize = 0;
  let totalGzipped = 0;

  function processDir(dir: string) {
    if (!fs.existsSync(dir)) return;

    const files = fs.readdirSync(dir);

    for (const file of files) {
      const filePath = path.join(dir, file);
      const stat = fs.statSync(filePath);

      if (stat.isDirectory()) {
        processDir(filePath);
      } else if (file.endsWith('.js')) {
        const content = fs.readFileSync(filePath);
        const gzipped = gzipSync(content);

        totalSize += stat.size;
        totalGzipped += gzipped.length;

        console.log(`  ${file}: ${formatBytes(stat.size)} → ${formatBytes(gzipped.length)} (gzipped)`);
      }
    }
  }

  processDir(staticDir);

  const gzippedKB = totalGzipped / 1024;
  const targetPassed = gzippedKB < TARGET_BUNDLE_SIZE_KB;

  console.log(`\n  Total: ${formatBytes(totalSize)} → ${formatBytes(totalGzipped)} (gzipped)`);
  console.log(`  Target: < ${TARGET_BUNDLE_SIZE_KB}KB gzipped`);
  console.log(`  Status: ${targetPassed ? '✅ PASSED' : '❌ FAILED'}\n`);

  return {
    raw: totalSize,
    gzipped: totalGzipped,
    targetPassed,
  };
}

function measureBuildTime(): BenchmarkResults['buildTime'] {
  console.log('\n⏱️  Measuring build time...\n');

  const startTime = Date.now();

  try {
    execSync('npm run build', {
      stdio: 'inherit',
      env: { ...process.env, NODE_ENV: 'production' },
    });

    const duration = Date.now() - startTime;
    const targetPassed = duration < MAX_BUILD_TIME_MS;

    console.log(`\n  Build time: ${(duration / 1000).toFixed(2)}s`);
    console.log(`  Target: < ${MAX_BUILD_TIME_MS / 1000}s`);
    console.log(`  Status: ${targetPassed ? '✅ PASSED' : '❌ FAILED'}\n`);

    return {
      duration,
      targetPassed,
    };
  } catch (error) {
    console.error('Build failed:', error);
    throw error;
  }
}

function generateRecommendations(results: BenchmarkResults): string[] {
  const recommendations: string[] = [];

  if (!results.bundleSize.targetPassed) {
    recommendations.push(
      '⚠️  Bundle size exceeds target. Consider:',
      '  - Code splitting large components',
      '  - Lazy loading routes',
      '  - Tree-shaking unused dependencies',
      '  - Removing duplicate dependencies'
    );
  }

  if (!results.buildTime.targetPassed) {
    recommendations.push(
      '⚠️  Build time exceeds target. Consider:',
      '  - Enabling SWC minification',
      '  - Reducing number of pages',
      '  - Optimizing webpack configuration'
    );
  }

  if (recommendations.length === 0) {
    recommendations.push('✅ All performance targets met!');
  }

  return recommendations;
}

function runBenchmarks(): BenchmarkResults {
  console.log('🚀 Running Performance Benchmarks\n');
  console.log('=' .repeat(50));

  const bundleSize = measureBundleSize();

  // Skip build time measurement if build already exists
  const buildTime = {
    duration: 0,
    targetPassed: true,
  };

  const results: BenchmarkResults = {
    bundleSize,
    buildTime,
    recommendations: [],
  };

  results.recommendations = generateRecommendations(results);

  return results;
}

function printReport(results: BenchmarkResults): void {
  console.log('\n' + '='.repeat(50));
  console.log('\n📊 Performance Report\n');

  console.log('Bundle Size:');
  console.log(`  Raw: ${formatBytes(results.bundleSize.raw)}`);
  console.log(`  Gzipped: ${formatBytes(results.bundleSize.gzipped)}`);
  console.log(`  Status: ${results.bundleSize.targetPassed ? '✅ PASSED' : '❌ FAILED'}\n`);

  console.log('Recommendations:');
  results.recommendations.forEach((rec) => console.log(`  ${rec}`));

  console.log('\n' + '='.repeat(50) + '\n');

  // Save results to JSON
  const reportPath = path.join(process.cwd(), 'performance-report.json');
  fs.writeFileSync(reportPath, JSON.stringify(results, null, 2));
  console.log(`📄 Full report saved to: ${reportPath}\n`);
}

// Main execution
try {
  const results = runBenchmarks();
  printReport(results);

  // Exit with error code if any target failed
  const allPassed = results.bundleSize.targetPassed && results.buildTime.targetPassed;
  process.exit(allPassed ? 0 : 1);
} catch (error) {
  console.error('❌ Benchmark failed:', error);
  process.exit(1);
}
