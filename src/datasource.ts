import { Observable, merge, from } from 'rxjs';
import flatten from 'lodash/flatten';
import { DataSourceWithBackend } from '@grafana/runtime';
import { DataQueryResponse, DataQueryRequest, toDataFrame } from '@grafana/data';
import { InfinityProvider } from './app/InfinityProvider';
import { SeriesProvider } from './app/SeriesProvider';
import { replaceVariables } from './app/InfinityQuery';
import { LegacyVariableProvider, InfinityVariableProvider, migrateLegacyQuery } from './app/variablesQuery';
import {
  InfinityQuery,
  GlobalInfinityQuery,
  VariableQuery,
  MetricFindValue,
  HealthCheckResult,
  HealthCheckResultStatus,
  InfinityInstanceSettings,
  InfinityDataSourceJSONOptions,
} from './types';

export class Datasource extends DataSourceWithBackend<InfinityQuery, InfinityDataSourceJSONOptions> {
  instanceSettings: InfinityInstanceSettings;
  constructor(iSettings: InfinityInstanceSettings) {
    super(iSettings);
    this.instanceSettings = iSettings;
  }
  private overrideWithGlobalQuery(t: InfinityQuery): InfinityQuery {
    if (
      t.type === 'global' &&
      t.global_query_id &&
      this.instanceSettings.jsonData.global_queries &&
      this.instanceSettings.jsonData.global_queries.length > 0
    ) {
      let matchingQuery = this.instanceSettings.jsonData.global_queries.find(
        (q: GlobalInfinityQuery) => q.id === t.global_query_id
      );
      t = matchingQuery ? matchingQuery.query : t;
    }
    return t;
  }
  testDatasource(): Promise<HealthCheckResult> {
    return new Promise((resolve, reject) => {
      if (
        this.instanceSettings.jsonData &&
        this.instanceSettings.jsonData.datasource_mode &&
        this.instanceSettings.jsonData.datasource_mode === 'basic'
      ) {
        resolve({ message: 'No checks required', status: HealthCheckResultStatus.Success });
      } else {
        if (this.instanceSettings.url) {
          resolve({ message: 'No checks performed', status: HealthCheckResultStatus.Success });
        } else {
          reject({ message: 'Missing URL', status: HealthCheckResultStatus.Failure });
        }
      }
    });
  }
  query(options: DataQueryRequest<InfinityQuery>): Observable<DataQueryResponse> {
    return new Observable<DataQueryResponse>(subscriber => {
      const targets = options.targets.filter((t: InfinityQuery) => t.hide !== true);
      targets.forEach(async t => {
        t = this.overrideWithGlobalQuery(t);
        switch (t.type) {
          case 'csv':
          case 'html':
          case 'json':
          case 'xml':
          case 'graphql':
            let iq = new InfinityProvider(replaceVariables(t, options.scopedVars), this.instanceSettings).query();
            from(iq).subscribe(r => subscriber.next({ data: [toDataFrame(r)] }));
            break;
          case 'series':
            const start = new Date(options.range.from.toDate()).getTime();
            const end = new Date(options.range.to.toDate()).getTime();
            let sq = new SeriesProvider(replaceVariables(t, options.scopedVars)).query(start, end);
            from(sq).subscribe(r => subscriber.next({ data: [toDataFrame(r)] }));
            break;
          case 'global':
            subscriber.error('Query not found');
            break;
          default:
            subscriber.error('Unknown Query Type');
            break;
        }
      });
    });
  }
  metricFindQuery(originalQuery: VariableQuery): Promise<MetricFindValue[]> {
    return new Promise(resolve => {
      let query = migrateLegacyQuery(originalQuery);
      switch (query.queryType) {
        case 'infinity':
          if (query.infinityQuery) {
            const infinityVariableProvider = new InfinityVariableProvider(query.infinityQuery, this.instanceSettings);
            infinityVariableProvider.query().then(res => {
              resolve(flatten(res));
            });
          } else {
            resolve([]);
          }
          break;
        case 'legacy':
        default:
          const legacyVariableProvider = new LegacyVariableProvider(query.query);
          legacyVariableProvider.query().then(res => {
            resolve(flatten(res));
          });
          break;
      }
    });
  }
}
