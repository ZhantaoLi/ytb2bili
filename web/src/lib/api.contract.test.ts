import { videoApi } from './api';
import type { ApiResponse, VideoFilesResponse } from '@/types';

type Equal<X, Y> = (<T>() => T extends X ? 1 : 2) extends <T>() => T extends Y ? 1 : 2
  ? true
  : false;
type Expect<T extends true> = T;

type GetVideoFilesReturn = ReturnType<typeof videoApi.getVideoFiles>;

type _GetVideoFilesMatchesBackendShape = Expect<
  Equal<GetVideoFilesReturn, Promise<ApiResponse<VideoFilesResponse>>>
>;
