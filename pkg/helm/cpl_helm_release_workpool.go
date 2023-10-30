/*
 * Copyright 2023 The Kubebb Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package helm

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubebb/core/api/v1alpha1"
)

var _ ReleaseWorkerPool = &WorkerPool{}

// ReleaseWorkerPool handle all helm action.we can mock it for testing
type ReleaseWorkerPool interface {
	// GetManifests is a synchronization function
	GetManifests(ctx context.Context, plan *v1alpha1.ComponentPlan, repo *v1alpha1.Repository, chartName string) (data string, err error)
	// InstallOrUpgrade is an asynchronous function.
	InstallOrUpgrade(ctx context.Context, plan *v1alpha1.ComponentPlan, repo *v1alpha1.Repository, chartName string) (rel *release.Release, doing bool, err error)
	// Uninstall is an asynchronous function.
	Uninstall(ctx context.Context, plan *v1alpha1.ComponentPlan) (doing bool, err error)
	// GetLastRelease is a synchronization function
	GetLastRelease(plan *v1alpha1.ComponentPlan) (rel *release.Release, err error)
	// RollBack is an asynchronous function
	RollBack(ctx context.Context, plan *v1alpha1.ComponentPlan) (rel *release.Release, doing bool, err error)
}

type WorkerPool struct {
	sync.Mutex
	cli           client.Client
	logger        logr.Logger
	installJobs   map[string]*installWorker   // key: jobkey()
	uninstallJobs map[string]*uninstallWorker // key: jobkey()
	rollbackJobs  map[string]*rollbackWorker  // key: jobkey()
	resultCache   cache.Store
	getter        map[string]genericclioptions.RESTClientGetter // key: getterKey()
}

func NewWorkerPool(logger logr.Logger, cli client.Client) *WorkerPool {
	return &WorkerPool{
		cli:           cli,
		logger:        logger,
		installJobs:   make(map[string]*installWorker),
		uninstallJobs: make(map[string]*uninstallWorker),
		rollbackJobs:  make(map[string]*rollbackWorker),
		resultCache:   cache.NewTTLStore(workCacheKey, 1*time.Hour),
		getter:        make(map[string]genericclioptions.RESTClientGetter),
	}
}

func workCacheKey(obj interface{}) (string, error) {
	installWorker, ok := obj.(*installWorker)
	if !ok {
		return "", fmt.Errorf("expected *installWorker, got %T", obj)
	}
	plan := installWorker.plan
	if plan == nil {
		return "", fmt.Errorf("expected non-nil plan, got nil")
	}
	return planCacheKey(plan), nil
}

func planCacheKey(plan *v1alpha1.ComponentPlan) string {
	return fmt.Sprintf("%s/%d", plan.GetUID(), plan.GetGeneration())
}

func (r *WorkerPool) GetLastRelease(plan *v1alpha1.ComponentPlan) (rel *release.Release, err error) {
	getter, err := r.getGetter(plan.Namespace, "")
	if err != nil {
		return nil, err
	}
	c, err := NewCoreHelmWrapper(getter, plan.Namespace, r.logger, r.cli, plan, nil, nil)
	if err != nil {
		return nil, err
	}
	return c.GetLastRelease()
}

func (r *WorkerPool) GetManifests(ctx context.Context, plan *v1alpha1.ComponentPlan, repo *v1alpha1.Repository, chartName string) (data string, err error) {
	getter, err := r.getGetterByPlan(plan)
	if err != nil {
		return "", err
	}
	c, err := NewCoreHelmWrapper(getter, plan.Namespace, r.logger, r.cli, plan, repo, nil)
	if err != nil {
		return "", err
	}
	return c.GetManifestsByDryRun(ctx, chartName)
}

func (r *WorkerPool) InstallOrUpgrade(ctx context.Context, plan *v1alpha1.ComponentPlan, repo *v1alpha1.Repository, chartName string) (rel *release.Release, isRunning bool, err error) {
	getter, err := r.getGetterByPlan(plan)
	if err != nil {
		return nil, false, err
	}
	r.Lock()
	defer r.Unlock()
	uninstallJob, ok := r.uninstallJobs[r.jobKey(plan)]
	if ok && uninstallJob.isRunning {
		return nil, true, errors.New("this helm release is uninstalling")
	}
	job, ok := r.installJobs[r.jobKey(plan)]
	if !ok || job.isExpired(plan, repo) {
		job = newInstallWorker(ctx, r.jobKey(plan), chartName, r.logger, plan, repo, getter, r.cli)
		r.installJobs[r.jobKey(plan)] = job
		if err = r.resultCache.Add(job); err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}
	item, exist, err := r.resultCache.GetByKey(planCacheKey(plan))
	if err != nil {
		return nil, false, err
	}
	if !exist {
		return nil, false, errors.New("cant find the plan result from cache, maybe restart operator when installing")
	}
	job, ok = item.(*installWorker)
	if !ok {
		return nil, false, fmt.Errorf("expected *installWorker, got %T", item)
	}
	return job.release, job.isRunning, job.err
}

func (r *WorkerPool) Uninstall(ctx context.Context, plan *v1alpha1.ComponentPlan) (doing bool, err error) {
	getter, err := r.getGetterByPlan(plan)
	if err != nil {
		return false, err
	}
	r.Lock()
	defer r.Unlock()
	job, ok := r.uninstallJobs[r.jobKey(plan)]
	if !ok || !job.isSame(plan) {
		r.uninstallJobs[r.jobKey(plan)] = newUnInstallWorker(ctx, r.jobKey(plan), r.logger, plan, getter, r.cli)
		return true, nil
	}
	_, doing, err = job.GetResult()
	return
}
func (r *WorkerPool) RollBack(ctx context.Context, plan *v1alpha1.ComponentPlan) (rel *release.Release, doing bool, err error) {
	getter, err := r.getGetterByPlan(plan)
	if err != nil {
		return nil, false, err
	}
	r.Lock()
	defer r.Unlock()
	job, ok := r.rollbackJobs[r.jobKey(plan)]
	if !ok || !job.isSame(plan) {
		r.rollbackJobs[r.jobKey(plan)] = newRollBackWorker(ctx, r.jobKey(plan), r.logger, plan, getter)
		return nil, true, nil
	}
	return job.GetResult()
}

func (r *WorkerPool) getterKey(ns, impersonateUserName string) string {
	return ns + "/" + impersonateUserName
}

func (r *WorkerPool) jobKey(plan *v1alpha1.ComponentPlan) string {
	return plan.GetNamespace() + "/" + plan.GetReleaseName()
}

func (r *WorkerPool) getGetterByPlan(plan *v1alpha1.ComponentPlan) (getter genericclioptions.RESTClientGetter, err error) {
	return r.getGetter(plan.Namespace, plan.Spec.Creator)
}

func (r *WorkerPool) getGetter(ns, impersonateUserName string) (getter genericclioptions.RESTClientGetter, err error) {
	key := r.getterKey(ns, impersonateUserName)
	r.Lock()
	defer r.Unlock()
	if getter, ok := r.getter[key]; ok {
		return getter, nil
	}

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get config for in-cluster REST client: %w", err)
	}
	var impersonate string
	if impersonateUserName != "" {
		cfg.Impersonate.UserName = impersonateUserName
		impersonate = impersonateUserName
	}
	getter = &genericclioptions.ConfigFlags{
		APIServer:   &cfg.Host,
		CAFile:      &cfg.CAFile,
		BearerToken: &cfg.BearerToken,
		Namespace:   &ns,
		Impersonate: &impersonate,
	}
	r.getter[key] = getter
	return getter, nil
}

type baseWorker struct {
	name      string
	cancel    context.CancelFunc
	logger    logr.Logger
	plan      *v1alpha1.ComponentPlan
	repo      *v1alpha1.Repository
	chartName string
	status    release.Status
	release   *release.Release
	isRunning bool
	startTime metav1.Time
	err       error
}

func (w *baseWorker) GetResult() (rel *release.Release, isRuuing bool, err error) {
	return w.release, w.isRunning, w.err
}

func (w *baseWorker) logKV() []interface{} {
	res := []interface{}{"workername", w.name}
	if w.plan != nil {
		res = append(res, "planUID", string(w.plan.UID))
		res = append(res, "planGeneration", strconv.FormatInt(w.plan.Generation, 10))
	}
	return res
}

type installWorker struct {
	baseWorker
}

func newInstallWorker(ctx context.Context, name, chartName string, logger logr.Logger, plan *v1alpha1.ComponentPlan, repo *v1alpha1.Repository, getter genericclioptions.RESTClientGetter, cli client.Client) *installWorker {
	w := &installWorker{
		baseWorker: baseWorker{
			name:      name,
			logger:    logger,
			plan:      plan,
			chartName: chartName,
			repo:      repo,
			isRunning: true,
		},
	}
	subCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	go func() {
		w.logger.V(1).Info("start install worker", w.logKV()...)
		w.startTime = metav1.Now()
		defer func() {
			cost := time.Since(w.startTime.Time)
			w.logger.V(1).Info(fmt.Sprintf("stop install worker, cost %s", cost), w.logKV()...)
			w.isRunning = false
		}()
		w.status = release.StatusPendingInstall
		c, err := NewCoreHelmWrapper(getter, plan.Namespace, w.logger, cli, plan, w.repo, nil)
		if err != nil {
			w.status = release.StatusFailed
			w.err = err
			return
		}
		w.release, w.err = c.InstallOrUpgrade(subCtx, w.chartName)
		if w.err != nil {
			w.status = release.StatusFailed
		} else {
			w.status = release.StatusDeployed
		}
	}()
	return w
}

func (w *installWorker) isExpired(plan *v1alpha1.ComponentPlan, repo *v1alpha1.Repository) (expired bool) {
	defer func() {
		if expired && w.isRunning {
			w.logger.Info(fmt.Sprintf("cancel expired install worker, cost %s", time.Since(w.startTime.Time)), "workername", w.name)
			w.cancel()
		}
	}()
	isPlanSame := w.plan.GetUID() == plan.GetUID()
	isRepoSame := w.repo.GetUID() == repo.GetUID()
	isPlanGenerationUpdated := plan.GetGeneration() > w.plan.GetGeneration()
	if isPlanSame && isRepoSame && isPlanGenerationUpdated {
		return true
	}
	if !isPlanSame || !isRepoSame {
		// TODO need more logic
		return true
	}
	return false
}

type uninstallWorker struct {
	baseWorker
}

func newUnInstallWorker(ctx context.Context, name string, logger logr.Logger, plan *v1alpha1.ComponentPlan, getter genericclioptions.RESTClientGetter, cli client.Client) *uninstallWorker {
	w := &uninstallWorker{
		baseWorker: baseWorker{
			name:      name,
			logger:    logger,
			plan:      plan,
			isRunning: true,
		},
	}
	subCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	go func() {
		w.logger.V(1).Info("start uninstall worker", w.logKV()...)
		startTime := metav1.Now()
		defer func() {
			w.logger.V(1).Info(fmt.Sprintf("stop uninstall worker, cost %s", time.Since(startTime.Time)), w.logKV()...)
			w.isRunning = false
		}()
		w.status = release.StatusUninstalling
		c, err := NewCoreHelmWrapper(getter, plan.Namespace, w.logger, cli, plan, nil, nil)
		if err != nil {
			w.status = release.StatusFailed
			w.err = err
			return
		}
		w.err = c.Uninstall(subCtx)
		if w.err != nil {
			w.status = release.StatusFailed
		} else {
			w.status = release.StatusUninstalled
		}
	}()
	return w
}

func (w *baseWorker) isSame(plan *v1alpha1.ComponentPlan) (same bool) {
	isPlanSame := w.plan.GetUID() == plan.GetUID()
	isPlanGenerationSame := plan.GetGeneration() == w.plan.GetGeneration()
	if isPlanSame && isPlanGenerationSame {
		return true
	}
	return false
}

type rollbackWorker struct {
	baseWorker
}

func newRollBackWorker(ctx context.Context, name string, logger logr.Logger, plan *v1alpha1.ComponentPlan, getter genericclioptions.RESTClientGetter) *rollbackWorker {
	w := &rollbackWorker{
		baseWorker: baseWorker{
			name:      name,
			logger:    logger,
			plan:      plan,
			isRunning: true,
		},
	}
	subCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	go func() {
		w.logger.V(1).Info("start rollback worker", w.logKV()...)
		startTime := metav1.Now()
		defer func() {
			w.logger.V(1).Info(fmt.Sprintf("stop rollback worker, cost %s", time.Since(startTime.Time)), w.logKV()...)
			w.isRunning = false
		}()
		w.status = release.StatusUninstalling
		c, err := NewCoreHelmWrapper(getter, plan.Namespace, w.logger, nil, plan, nil, nil)
		if err != nil {
			w.status = release.StatusFailed
			w.err = err
			return
		}

		w.err = c.Rollback(subCtx)
		if w.err != nil {
			w.status = release.StatusFailed
			return
		}
		rel, err := c.GetLastRelease()
		if rel != nil && rel.Info != nil && strings.HasPrefix(rel.Info.Description, "Rollback to ") {
			w.release = rel
		}
		w.err = err
		w.status = release.StatusDeployed
	}()
	return w
}
